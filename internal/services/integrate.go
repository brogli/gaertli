package services

import (
	"context"
	"strings"

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

// shortRefName drops the namespace of a branch, remote branch or tag ref; ok is false for any
// other name.
func shortRefName(full string) (short string, ok bool) {
	for _, prefix := range []string{"refs/heads/", "refs/remotes/", "refs/tags/"} {
		if short, ok := strings.CutPrefix(full, prefix); ok {
			return short, true
		}
	}
	return "", false
}

// Merge implements core.API.
func (c *Core) Merge(ctx context.Context, id core.WorktreeID, ref string, stash bool) (core.IntegrateResult, error) {
	return c.integrate(ctx, id, ref, mergeVerb, stash)
}

// Rebase implements core.API.
func (c *Core) Rebase(ctx context.Context, id core.WorktreeID, ref string, stash bool) (core.IntegrateResult, error) {
	return c.integrate(ctx, id, ref, rebaseVerb, stash)
}

// integrate runs verb of ref into wt's current branch under wt's write lock,
// with local changes stashed around it if stash.
func (c *Core) integrate(ctx context.Context, id core.WorktreeID, ref string, verb integrateVerb, stash bool) (core.IntegrateResult, error) {
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

// refuseIntegrate is the refusal before git runs: an operation in progress
// or a detached HEAD.
func refuseIntegrate(ctx context.Context, wt *session.Worktree) error {
	if op := operation(wt.GitDir); op != "" {
		return core.ErrorfArgs(core.GitFailed, "operationInProgress", map[string]string{"operation": op}, "A %s is in progress. Continue or abort it first.", op)
	}
	_, detached, err := headBranch(ctx, wt.Runner, wt.Path)
	if err != nil {
		return err
	}
	if detached {
		return core.ErrorfArgs(core.GitFailed, "detachedHead", nil, "HEAD is detached. Check out a branch first.")
	}
	return nil
}

// integrateArg is ref as git is given it: its short name when that names
// ref alone, so a merge message reads as on the command line, else the full
// ref.
func integrateArg(ctx context.Context, wt *session.Worktree, ref string) (string, error) {
	short, ok := shortRefName(ref)
	if !ok || strings.HasPrefix(short, "-") {
		return ref, nil
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"rev-parse", "--verify", "--quiet", "--symbolic-full-name", short},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	if strings.TrimSpace(string(result.Stdout)) == ref {
		return short, nil
	}
	return ref, nil
}

// integrateOutcome tells what a successful verb did, from HEAD before and
// after. A merge that moved HEAD to arg's commit fast-forwarded.
func integrateOutcome(ctx context.Context, wt *session.Worktree, rebased bool, before, after, arg string) (core.IntegrateOutcome, error) {
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
