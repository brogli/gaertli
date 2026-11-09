package services

import (
	"context"
	"strings"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// integrateVerb is what Merge and Rebase differ in: the git subcommand with
// the flags that keep the user's config out of it.
type integrateVerb struct {
	name  string // "merge" or "rebase"
	flags []string
}

var (
	mergeVerb  = integrateVerb{"merge", []string{"-s", "ort", "--no-edit", "--ff", "--commit", "--no-squash", "--no-autostash"}}
	rebaseVerb = integrateVerb{"rebase", []string{"--merge", "--no-rebase-merges", "--no-update-refs", "--no-autostash"}}
)

// Merge implements core.API.
func (c *Core) Merge(ctx context.Context, id core.WorktreeID, ref core.RefName, stash bool) (core.IntegrateResult, error) {
	return c.integrate(ctx, id, ref, mergeVerb, stash)
}

// Rebase implements core.API.
func (c *Core) Rebase(ctx context.Context, id core.WorktreeID, ref core.RefName, stash bool) (core.IntegrateResult, error) {
	return c.integrate(ctx, id, ref, rebaseVerb, stash)
}

// integrate runs verb of ref into wt's current branch under wt's write lock,
// with local changes stashed around it if stash.
func (c *Core) integrate(ctx context.Context, id core.WorktreeID, ref core.RefName, verb integrateVerb, stash bool) (core.IntegrateResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.IntegrateResult{}, err
	}
	wt.Lock()
	defer wt.Unlock()

	if err := refuseIntegrate(ctx, wt); err != nil {
		return core.IntegrateResult{}, err
	}
	arg, err := integrateArg(ctx, wt, ref)
	if err != nil {
		return core.IntegrateResult{}, err
	}
	before, err := revParse(ctx, wt.Runner, wt.Path, "HEAD")
	if err != nil {
		return core.IntegrateResult{}, err
	}

	run := func() error { return runIntegrate(ctx, wt, verb, arg) }
	if stash {
		err = withStash(ctx, wt, verb.name, run)
	} else {
		err = run()
	}
	if err != nil {
		return core.IntegrateResult{}, err
	}

	after, err := revParse(ctx, wt.Runner, wt.Path, "HEAD")
	if err != nil {
		return core.IntegrateResult{}, err
	}
	outcome, err := integrateOutcome(ctx, wt, verb.name == rebaseVerb.name, before, after, arg)
	return core.IntegrateResult{Outcome: outcome}, err
}

// runIntegrate runs verb of arg in wt, classifying a failure.
func runIntegrate(ctx context.Context, wt *session.Worktree, verb integrateVerb, arg string) error {
	args := append([]string{verb.name}, verb.flags...)
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, NoEditor: true, Timeout: operationTimeout,
		Args: append(args, "--end-of-options", arg),
	})
	if err != nil {
		return classifyDirtyIntegrate(stopOf(ctx, wt, err), result, verb.name)
	}
	return nil
}

// refuseIntegrate is integrateRefusal for wt's operation and HEAD.
func refuseIntegrate(ctx context.Context, wt *session.Worktree) error {
	op := operation(wt.GitDir)
	detached := false
	if op == "" {
		var err error
		if _, detached, err = headBranch(ctx, wt.Runner, wt.Path); err != nil {
			return err
		}
	}
	if refusal := integrateRefusal(op, detached); refusal != nil {
		return refusal
	}
	return nil
}

// integrateArg is ref as git is given it: its short name when that names
// ref alone, so a merge message reads as on the command line, else the full
// ref.
func integrateArg(ctx context.Context, wt *session.Worktree, ref core.RefName) (string, error) {
	short := ref.Short()
	if strings.HasPrefix(short, "-") {
		return string(ref), nil
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"rev-parse", "--verify", "--quiet", "--symbolic-full-name", short},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	if core.RefName(strings.TrimSpace(string(result.Stdout))) == ref {
		return short, nil
	}
	return string(ref), nil
}

// integrateOutcome tells what a successful verb did, from HEAD before and
// after. A merge that moved HEAD to arg's commit fast-forwarded.
func integrateOutcome(ctx context.Context, wt *session.Worktree, rebased bool, before, after core.SHA, arg string) (core.IntegrateOutcome, error) {
	switch {
	case before == after:
		return core.IntegrateUpToDate, nil
	case rebased:
		return core.IntegrateRebased, nil
	}
	target, err := revParse(ctx, wt.Runner, wt.Path, arg+"^{commit}")
	switch {
	case err != nil:
		return "", err
	case target == after:
		return core.IntegrateFastForward, nil
	}
	return core.IntegrateMerged, nil
}

// ContinueOperation implements core.API.
func (c *Core) ContinueOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--continue")
}

// AbortOperation implements core.API.
func (c *Core) AbortOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--abort")
}

// SkipOperation implements core.API.
func (c *Core) SkipOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--skip")
}

// runOperationVerb runs flag against the in-progress operation, whose kind
// is itself the git subcommand. Only a skippable one is skipped. A refusal
// stands in for git's own, as GitFailed. Continue and skip may commit with a
// message, so no editor waits; abort never commits.
func (c *Core) runOperationVerb(ctx context.Context, id core.WorktreeID, flag string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // continue/abort/skip write this worktree's index and files
	defer wt.Unlock()

	kind := operation(wt.GitDir)
	switch {
	case kind == "":
		return core.ErrorfArgs(core.GitFailed, "noOperation", nil, "No merge, rebase, cherry-pick or revert is in progress.")
	case flag == "--skip" && !skippable(kind):
		return core.ErrorfArgs(core.GitFailed, "operationNotSkippable", map[string]string{"operation": string(kind)}, "A %s can't be skipped; only a cherry-pick or revert can.", kind)
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, NoEditor: flag != "--abort", Timeout: operationTimeout, Args: []string{string(kind), flag}})
	return stopOf(ctx, wt, err)
}

// stopOf returns err from an operation step as it is, unless the step
// left an operation in progress: then it is a stop left for continue or
// abort (asStop). The status read ignores ctx's end, so a step that timed
// out is still told apart.
func stopOf(ctx context.Context, wt *session.Worktree, err error) error {
	if err == nil || operation(wt.GitDir) == "" {
		return err
	}
	st, statusErr := readHeadStatus(context.WithoutCancel(ctx), wt)
	return asStop(err, statusErr == nil && st.conflicted)
}

// operationTimeout bounds a rebase, or one --continue/--abort/--skip of an
// operation: replaying many commits, or a hook or signing prompt on the
// way, outlasts the runner's default, and a kill mid-way leaves the
// operation half done.
const operationTimeout = 10 * time.Minute

// remapOperationConflict reclassifies git's failure as Code Conflicted while
// an operation is still in progress under gitDir.
func remapOperationConflict(err error, gitDir string) error {
	return remapIfPending(err, operation(gitDir) != "")
}

// integrateStep runs Pull's merge/rebase step. Caller holds wt's write lock.
func integrateStep(ctx context.Context, wt *session.Worktree, integrateArgs []string) error {
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: integrateArgs})
	return classifyDirtyIntegrate(remapOperationConflict(err, wt.GitDir), result, "pull")
}

// dirtyIntegrateReasons are the integrate step's refusals to start because
// of local changes: rebase refuses on any staged or unstaged change, a merge
// only on a change to a file the incoming commits also touch. Nothing
// started, so no state file marks them the way remapOperationConflict needs.
var dirtyIntegrateReasons = []string{
	"error: cannot rebase: You have unstaged changes.",
	"error: cannot rebase: Your index contains uncommitted changes.",
	"error: Your local changes to the following files would be overwritten by merge:",
}

// classifyDirtyIntegrate reclassifies an integrate refusal matching
// dirtyIntegrateReasons as Code DirtyWorktree, keyed by verb ("pull",
// "merge" or "rebase").
func classifyDirtyIntegrate(err error, result git.Result, verb string) error {
	if ce := gitFailure(err); ce != nil && containsAny(string(result.Stderr), dirtyIntegrateReasons) {
		switch verb {
		case "merge":
			ce.Retag(core.DirtyWorktree, "mergeBlockedByChanges", nil, "Local changes are in the way of the merge.")
		case "rebase":
			ce.Retag(core.DirtyWorktree, "rebaseBlockedByChanges", nil, "Local changes are in the way of the rebase.")
		default:
			ce.Retag(core.DirtyWorktree, "pullBlockedByChanges", nil, "Local changes are in the way of the pull.")
		}
	}
	return err
}

// integrateWithStash is integrateStep with local changes stashed around it.
func integrateWithStash(ctx context.Context, wt *session.Worktree, integrateArgs []string) error {
	return withStash(ctx, wt, "pull", func() error { return integrateStep(ctx, wt, integrateArgs) })
}

// untouched reports whether wt has no operation in progress, HEAD at head, and the index and
// tracked files as HEAD has them.
func untouched(ctx context.Context, wt *session.Worktree, head core.SHA) bool {
	if operation(wt.GitDir) != "" {
		return false
	}
	now, err := revParse(ctx, wt.Runner, wt.Path, "HEAD")
	return err == nil && now == head && matchesHead(ctx, wt)
}

// matchesHead reports whether the index and the tracked files equal HEAD.
// The refresh drops files whose stat alone changed; a submodule's checkout,
// which stash push never moves, does not count.
func matchesHead(ctx context.Context, wt *session.Worktree) bool {
	_, _ = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"update-index", "-q", "--refresh"}})
	_, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Args: []string{"diff-index", "--quiet", "--ignore-submodules", "HEAD", "--"}})
	return err == nil
}
