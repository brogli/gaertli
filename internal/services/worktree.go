package services

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// ListWorktrees implements core.API.
func (c *Core) ListWorktrees(ctx context.Context, path core.RepoPath) ([]core.Worktree, error) {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	worktrees, err := worktreeList(ctx, c.discover, toplevel, false)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxStatusRuns)
	for i := range worktrees {
		w := &worktrees[i]
		if w.Missing || w.Bare {
			continue
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			w.Dirty = c.isDirty(ctx, w.Path)
		})
	}
	wg.Wait()
	return worktrees, nil
}

// maxStatusRuns bounds how many status calls ListWorktrees runs at once.
const maxStatusRuns = 4

// isDirty reports whether dir has any change, untracked files included. A
// run error counts as clean: this is display data, and the error surfaces
// when the worktree is opened.
func (c *Core) isDirty(ctx context.Context, dir string) bool {
	result, err := c.discover.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"status", "--porcelain=v2", "-z", "--untracked-files=all"},
	})
	return err == nil && len(result.Stdout) > 0
}

// MainWorktree implements core.API.
func (c *Core) MainWorktree(ctx context.Context, gitDir core.RepoPath) (core.Worktree, error) {
	worktrees, err := worktreeList(ctx, c.discover, string(gitDir), false)
	if err != nil {
		return core.Worktree{}, session.RemapNotARepo(err, gitDir)
	}
	for _, wt := range worktrees {
		if !wt.Bare && !wt.Missing {
			return wt, nil
		}
	}
	return core.Worktree{}, core.ErrorfArgs(core.NotARepo, "noWorktreeLeft", map[string]string{"path": string(gitDir)}, "%s has no worktree left", gitDir)
}

// AddWorktree implements core.API.
func (c *Core) AddWorktree(ctx context.Context, path core.RepoPath, target, branch, start string, newBranch bool) (core.Worktree, error) {
	wt, err := c.sessions.Open(ctx, path) // path is an existing worktree of the repo
	if err != nil {
		return core.Worktree{}, err
	}
	wt.Repo.Lock() // worktree add writes repo-wide refs, like creating a branch
	defer wt.Repo.Unlock()

	args := []string{"worktree", "add", "--", target, branch}
	if newBranch {
		if err := validateBranchName(ctx, wt.Runner, wt.Path, branch); err != nil { // worktree add -b's name argument is not protected by --
			return core.Worktree{}, err
		}
		// git creates -b's branch before it finds target taken, and leaves
		// the branch behind; refuse first, in git's own words.
		if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
			return core.Worktree{}, core.ErrorfArgs(core.GitFailed, "worktreeExists", map[string]string{"path": target}, "'%s' already exists", target)
		}
		args = []string{"worktree", "add", "-b", branch, "--", target}
		if start != "" { // none: git branches from HEAD
			args = append(args, start)
		}
	}
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args}); err != nil {
		return core.Worktree{}, err
	}
	return describeWorktree(ctx, wt, target)
}

// describeWorktree returns target's row from wt's repo's worktree list.
func describeWorktree(ctx context.Context, wt *session.Worktree, target string) (core.Worktree, error) {
	worktrees, err := worktreeList(ctx, wt.Runner, wt.Path, false)
	if err != nil {
		return core.Worktree{}, err
	}
	for _, row := range worktrees {
		if row.Path == target {
			return row, nil // just created: clean by construction
		}
	}
	return core.Worktree{}, core.Errorf(core.Internal, "worktree %q not found after creation", target)
}

// RemoveWorktree implements core.API.
func (c *Core) RemoveWorktree(ctx context.Context, path core.RepoPath, target string, force bool) error {
	wt, err := c.sessions.Open(ctx, path) // path is an existing worktree of the repo
	if err != nil {
		return err
	}
	wt.Repo.Lock() // worktree remove is a repo-wide one-off
	defer wt.Repo.Unlock()

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "--", target)
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
	return remapDirtyWorktree(err, result, target)
}

// remapDirtyWorktree reclassifies remove's "contains modified or untracked
// files" refusal (exit 128) as Code DirtyWorktree, so the caller can confirm
// the loss and retry forced. A locked worktree's refusal stays GitFailed:
// force does not fix it.
func remapDirtyWorktree(err error, result git.Result, target string) error {
	if ce := gitFailure(err); ce != nil && result.ExitCode == 128 && bytes.Contains(result.Stderr, []byte("contains modified or untracked files")) {
		ce.Retag(core.DirtyWorktree, "worktreeDirty", map[string]string{"path": target}, "%s has uncommitted changes.", target)
	}
	return err
}

// PruneWorktrees implements core.API.
func (c *Core) PruneWorktrees(ctx context.Context, path core.RepoPath) error {
	wt, err := c.sessions.Open(ctx, path)
	if err != nil {
		return err
	}
	wt.Repo.Lock() // worktree prune is a repo-wide one-off
	defer wt.Repo.Unlock()

	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"worktree", "prune"}})
	return err
}

// worktreeList runs git worktree list in dir and parses it.
func worktreeList(ctx context.Context, r git.Runner, dir string, background bool) ([]core.Worktree, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: background, Args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if err != nil {
		return nil, err
	}
	return parseWorktreeRecords(result.Stdout), nil
}

// parseWorktreeRecords parses "git worktree list --porcelain -z" output
// (each field is its own NUL-terminated "key[ value]"; an empty field ends
// a record). git lists the main worktree first and porcelain has no marker
// for it, so Main is set from position alone. Dirty is left to the caller.
func parseWorktreeRecords(out []byte) []core.Worktree {
	var worktrees []core.Worktree
	for block := range bytes.SplitSeq(out, []byte{0, 0}) {
		if len(block) == 0 {
			continue
		}
		worktrees = append(worktrees, parseWorktreeRecord(block, len(worktrees) == 0))
	}
	return worktrees
}

func parseWorktreeRecord(block []byte, main bool) core.Worktree {
	wt := core.Worktree{Main: main}
	for line := range bytes.SplitSeq(block, []byte{0}) {
		key, value, _ := strings.Cut(string(line), " ")
		switch key {
		case "worktree":
			wt.Path = value
		case "HEAD":
			wt.SHA = value
		case "branch":
			wt.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "locked":
			wt.Locked = true
		case "prunable":
			wt.Missing = true
		case "bare":
			wt.Bare = true
		}
	}
	return wt
}
