package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// todoEditor is GIT_SEQUENCE_EDITOR for a todo prepared in the file
// GAERTLI_TODO_FILE names. git runs the value through its own shell,
// appending "$@" (its todo file); read, printf and the redirects are shell
// builtins, so it needs nothing beyond the sh git runs hooks with.
const todoEditor = `while IFS= read -r line; do printf '%s\n' "$line"; done < "$GAERTLI_TODO_FILE" >`

// selection is what SquashCheck, Squash and Fixup learn about the commits
// to fold.
type selection struct {
	head     core.SHA        // HEAD's commit before any rewrite
	upstream bool            // the branch has an upstream that exists
	chain    []chainCommit   // lineOf: from the oldest selected commit up to HEAD
	selected map[string]bool // the selected shas
	blocker  *core.Error     // why they cannot be folded; nil if they can
}

type chainCommit struct {
	sha, short string
	parents    []string
}

// oldest is the selected commit the others fold into; valid without a
// blocker.
func (s selection) oldest() chainCommit { return s.chain[0] }

// inChainOrder lists the selected shas oldest first.
func (s selection) inChainOrder() []string {
	var shas []string
	for _, c := range s.chain {
		if s.selected[c.sha] {
			shas = append(shas, c.sha)
		}
	}
	return shas
}

// SquashCheck implements core.API.
func (c *Core) SquashCheck(ctx context.Context, id core.WorktreeID, shas []core.SHA) (core.SquashCheck, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.SquashCheck{}, err
	}
	sel, err := inspectSelection(ctx, wt, shas)
	if err != nil || sel.blocker != nil {
		return core.SquashCheck{Blocker: sel.blocker}, err
	}
	pushed, err := sel.pushed(ctx, wt)
	if err != nil {
		return core.SquashCheck{}, err
	}
	message, err := combinedMessage(ctx, wt, sel.inChainOrder())
	return core.SquashCheck{Pushed: pushed, Message: message}, err
}

// Squash implements core.API.
func (c *Core) Squash(ctx context.Context, id core.WorktreeID, shas []core.SHA, message string) (core.SquashResult, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return core.SquashResult{}, core.Errorf(core.Internal, "squash needs a message")
	}
	return c.fold(ctx, id, shas, message)
}

// Fixup implements core.API.
func (c *Core) Fixup(ctx context.Context, id core.WorktreeID, shas []core.SHA) (core.SquashResult, error) {
	return c.fold(ctx, id, shas, "")
}

// fold is Squash's and Fixup's shared body: message "" keeps the oldest
// commit's message.
func (c *Core) fold(ctx context.Context, id core.WorktreeID, shas []core.SHA, message string) (core.SquashResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.SquashResult{}, err
	}
	wt.Lock() // the rebase writes this worktree's HEAD, index and files
	defer wt.Unlock()

	sel, err := inspectSelection(ctx, wt, shas)
	if err != nil {
		return core.SquashResult{}, err
	}
	if sel.blocker != nil {
		return core.SquashResult{}, sel.blocker
	}
	source := sel.oldest().sha // the commit the new one takes its message and author from
	if message != "" {
		if source, err = messageCarrier(ctx, wt, source, message); err != nil {
			return core.SquashResult{}, err
		}
	}
	if err := rebaseWithTodo(ctx, wt, sel, source); err != nil {
		return core.SquashResult{}, err
	}
	return core.SquashResult{PreviousHead: sel.head}, nil
}

// inspectSelection reads shas' chain and the worktree's state, and names
// the first blocker. The shas go into the rebase todo, where git would not
// refuse a malformed one, so they are validated first.
func inspectSelection(ctx context.Context, wt *session.Worktree, shas []core.SHA) (selection, error) {
	for _, sha := range shas {
		if err := sha.Validate(); err != nil {
			return selection{}, err
		}
	}
	selected := map[string]bool{}
	for _, sha := range shas {
		selected[string(sha)] = true
	}
	if len(selected) < 2 {
		return selection{}, core.Errorf(core.Internal, "squash needs two or more commits, got %d", len(selected))
	}
	st, err := readHeadStatus(ctx, wt)
	if err != nil {
		return selection{}, err
	}
	chain, err := lineOf(ctx, wt, shaArgs(shas))
	if err != nil {
		return selection{}, err
	}
	sel := selection{head: st.head, upstream: st.upstream, chain: chain, selected: selected}
	sel.blocker = sel.firstBlocker(operation(wt.GitDir), st)
	return sel, nil
}

// firstBlocker is why the selection cannot be folded, in the order
// SquashCheck documents; nil if it can.
func (s selection) firstBlocker(op core.OperationKind, st headStatus) *core.Error {
	switch {
	case op != "":
		return core.ErrorfArgs(core.GitFailed, "squashOperationInProgress", map[string]string{"operation": string(op)}, "a %s is in progress", op)
	case st.detached:
		return core.ErrorfArgs(core.GitFailed, "squashDetached", nil, "HEAD is detached; squash and fixup need the current branch")
	case len(s.inChainOrder()) != len(s.selected):
		return core.ErrorfArgs(core.GitFailed, "squashNotOnBranch", nil, "a selected commit is not on the current branch")
	}
	for _, c := range s.chain {
		if len(c.parents) > 1 {
			return core.ErrorfArgs(core.GitFailed, "squashMergeInTheWay", map[string]string{"sha": c.short}, "the merge commit %s is in the way", c.short)
		}
	}
	if st.dirty {
		return core.ErrorfArgs(core.DirtyWorktree, "squashUncommittedChanges", nil, "there are uncommitted changes; commit or stash them first")
	}
	return nil
}

type headStatus struct {
	head               core.SHA
	detached, upstream bool
	dirty              bool // tracked files have staged, unstaged or conflicted changes
	conflicted         bool
}

// readHeadStatus reads HEAD, whether it is detached, whether its branch's
// upstream exists, and whether tracked files changed, in one status call.
func readHeadStatus(ctx context.Context, wt *session.Worktree) (headStatus, error) {
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=no"},
	})
	if err != nil {
		return headStatus{}, err
	}
	var st headStatus
	for _, rec := range nulRecords(result.Stdout) {
		switch {
		case strings.HasPrefix(rec, "# branch.oid "):
			st.head = core.SHA(strings.TrimPrefix(rec, "# branch.oid "))
		case rec == "# branch.head (detached)":
			st.detached = true
		case strings.HasPrefix(rec, "# branch.ab "): // only when the upstream exists
			st.upstream = true
		case strings.HasPrefix(rec, "u "):
			st.dirty, st.conflicted = true, true
		case !strings.HasPrefix(rec, "# "):
			st.dirty = true
		}
	}
	return st, nil
}

// lineOf lists shas' octopus merge base and every commit after it up to
// HEAD, oldest first. When shas lie on one line below HEAD, the base is the
// oldest of them and the list holds them all; nil when they share no
// ancestor. base^@ is all of base's parents, none for a root commit.
func lineOf(ctx context.Context, wt *session.Worktree, shas []string) ([]chainCommit, error) {
	args := append([]string{"merge-base", "--octopus", "--end-of-options"}, shas...)
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Args: args})
	if quietFailure(err, result) {
		return nil, nil // no common ancestor
	}
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(string(result.Stdout))
	result, err = wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"rev-list", "--reverse", "--no-commit-header", "--format=%H %h %P", "HEAD", "--not", base + "^@"},
	})
	if err != nil {
		return nil, err
	}
	var chain []chainCommit
	for line := range strings.Lines(string(result.Stdout)) {
		f := strings.Fields(line)
		chain = append(chain, chainCommit{sha: f[0], short: f[1], parents: f[2:]})
	}
	return chain, nil
}

// pushed reports whether the oldest selected commit, and so every rewritten
// commit after it, is already on the branch's upstream.
func (s selection) pushed(ctx context.Context, wt *session.Worktree) (bool, error) {
	if !s.upstream {
		return false, nil
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"merge-base", "--is-ancestor", s.oldest().sha, "@{upstream}"},
	})
	if gitExited(err, result, 1) {
		return false, nil
	}
	return err == nil, err
}

// combinedMessage is shas' messages in the given order, a blank line
// between them.
func combinedMessage(ctx context.Context, wt *session.Worktree, shas []string) (string, error) {
	args := append([]string{"rev-list", "--no-walk=unsorted", "--no-commit-header", "--format=%B%x00"}, shas...)
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Args: args})
	if err != nil {
		return "", err
	}
	records := strings.Split(string(result.Stdout), "\x00")
	messages := make([]string, 0, len(shas))
	for _, rec := range records[:len(records)-1] { // the last one is the final newline
		messages = append(messages, strings.TrimSpace(rec))
	}
	return strings.Join(messages, "\n\n"), nil
}

// messageCarrier writes message into a commit authored like oldest, for
// the todo's amend to take both from.
func messageCarrier(ctx context.Context, wt *session.Worktree, oldest, message string) (string, error) {
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"rev-list", "-1", "--no-commit-header", "--date=raw", "--format=%an%x00%ae%x00%ad", oldest},
	})
	if err != nil {
		return "", err
	}
	author := strings.Split(strings.TrimSuffix(string(result.Stdout), "\n"), "\x00")
	result, err = wt.Runner.Run(ctx, git.Call{
		Dir:   wt.Path,
		Args:  []string{"commit-tree", "--no-gpg-sign", "-F", "-", oldest + "^{tree}"},
		Stdin: []byte(message + "\n"),
		Env:   []string{"GIT_AUTHOR_NAME=" + author[0], "GIT_AUTHOR_EMAIL=" + author[1], "GIT_AUTHOR_DATE=" + author[2]},
	})
	return strings.TrimSpace(string(result.Stdout)), err
}

// rebaseWithTodo runs git rebase -i on the chain with squashTodo's todo in
// place of git's own. The user's autostash and update-refs settings are
// overridden: a dirty tree is a blocker, and other branches stay put. A
// commit that joined the chain after it was read stops the rebase instead
// of being dropped (missingCommitsCheck). A failed amend stays in the todo
// (--reschedule-failed-exec, kept in the rebase's state), so --continue
// runs it again rather than finishing with the oldest commit's message.
func rebaseWithTodo(ctx context.Context, wt *session.Worktree, sel selection, source string) error {
	chain := make([]string, len(sel.chain))
	for i, c := range sel.chain {
		chain[i] = c.sha
	}
	todo, err := writeTodo(squashTodo(chain, sel.selected, source))
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(todo) }() // a leftover temp file is harmless
	upstream := "--root"
	if parents := sel.oldest().parents; len(parents) > 0 {
		upstream = parents[0]
	}
	_, err = wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, NoEditor: true, Timeout: operationTimeout,
		Args: []string{
			"-c", "rebase.missingCommitsCheck=error",
			"rebase", "--interactive", "--no-autostash", "--no-update-refs", "--empty=keep",
			"--reschedule-failed-exec", upstream,
		},
		Env: []string{"GIT_SEQUENCE_EDITOR=" + todoEditor, "GAERTLI_TODO_FILE=" + filepath.ToSlash(todo)},
	})
	return stopOf(ctx, wt, err)
}

// writeTodo writes todo to a temporary file for todoEditor to read.
func writeTodo(todo string) (string, error) {
	f, err := os.CreateTemp("", "gaertli-todo-*")
	if err == nil {
		_, err = f.WriteString(todo)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return "", core.Errorf(core.Internal, "writing the rebase todo: %v", err)
	}
	return f.Name(), nil
}

// squashTodo is the rebase todo for chain (oldest first) that folds the
// selected commits into chain[0]: pick it, fix up the other selected ones
// in their order, amend the result with source's message and author, then
// pick the rest in their order. --cleanup=verbatim keeps the message
// exactly, whatever commit.cleanup says; the amend is in the todo, so it
// also runs after a stop and --continue.
func squashTodo(chain []string, selected map[string]bool, source string) string {
	fold := []string{"pick " + chain[0]}
	var rest []string
	for _, sha := range chain[1:] {
		if selected[sha] {
			fold = append(fold, "fixup "+sha)
		} else {
			rest = append(rest, "pick "+sha)
		}
	}
	fold = append(fold, "exec git commit --amend --no-verify --allow-empty --cleanup=verbatim --quiet -C "+source)
	return strings.Join(append(fold, rest...), "\n") + "\n"
}
