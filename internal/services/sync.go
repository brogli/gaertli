package services

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/jobs"
	"github.com/brogli/nebegit/internal/session"
)

// remoteTimeout is git.Call's own safety-net Timeout for a remote call,
// generous on purpose: the job's own cancel through ctx is the real limit.
const remoteTimeout = time.Hour

// runWithProgress runs a --progress git.Call live, reporting every parsed
// line. A line with no percent keeps the last known percent and updates
// only the message, so a status line between two percent phases never
// blanks the job's progress bar.
func runWithProgress(ctx context.Context, r git.Runner, dir string, args []string, progress jobs.Progress) (git.Result, error) {
	percent := -1
	return r.StreamErr(ctx, git.Call{Dir: dir, Args: args, Timeout: remoteTimeout}, func(line []byte) error {
		if p, message, ok := git.ParseProgressLine(line); ok {
			percent = p
			progress(percent, message)
		} else {
			progress(percent, string(line))
		}
		return nil
	})
}

// Fetch implements core.API.
func (c *Core) Fetch(ctx context.Context, id core.WorktreeID, remote string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	args := []string{"fetch", "--prune", "--progress"}
	if remote == "" {
		args = append(args, "--all")
	} else {
		args = append(args, remote)
	}
	jobID := wt.Repo.Jobs().Start("fetch", func(ctx context.Context, progress jobs.Progress) (any, error) {
		result, err := runWithProgress(ctx, wt.Runner, wt.Path, args, progress)
		return nil, remapAuthFailed(err, result)
	})
	return string(jobID), nil
}

// Pull implements core.API.
func (c *Core) Pull(ctx context.Context, id core.WorktreeID, mode core.PullMode) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	integrateArgs, err := pullIntegrateArgs(mode)
	if err != nil {
		return "", err
	}
	jobID := wt.Repo.Jobs().Start("pull", func(ctx context.Context, progress jobs.Progress) (any, error) {
		if err := fetchStep(ctx, wt, progress); err != nil {
			return nil, err
		}
		wt.Lock() // only the local integration step writes this worktree's index/files
		defer wt.Unlock()
		return nil, integrateStep(ctx, wt, integrateArgs)
	})
	return string(jobID), nil
}

// fetchStep is Pull's and PullWithStash's shared first step.
func fetchStep(ctx context.Context, wt *session.Worktree, progress jobs.Progress) error {
	result, err := runWithProgress(ctx, wt.Runner, wt.Path, []string{"fetch", "--prune", "--progress"}, progress)
	return remapAuthFailed(err, result)
}

// integrateStep runs Pull's merge/rebase step. Caller holds wt's write lock.
func integrateStep(ctx context.Context, wt *session.Worktree, integrateArgs []string) error {
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: integrateArgs})
	return classifyDirtyIntegrate(remapOperationConflict(err, wt.GitDir), result)
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
// dirtyIntegrateReasons as Code DirtyWorktree.
func classifyDirtyIntegrate(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && containsAny(string(result.Stderr), dirtyIntegrateReasons) {
		ce.Code = core.DirtyWorktree
	}
	return err
}

// containsAny reports whether s contains any of subs.
func containsAny(s string, subs []string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}

// stashMarker tags the subject of a stash entry PullWithStash made, so
// Summary's pullStash tells it apart from a stash made outside the app.
const stashMarker = "nebegit: local changes before pull"

// PullWithStash implements core.API. git stash push exits 0 without a new
// entry when there is nothing to save, so refs/stash is read before and
// after the push: unchanged means nothing to pop; changed names this call's
// own entry, which is popped by its commit, never as a bare "stash pop"
// that could take someone else's.
func (c *Core) PullWithStash(ctx context.Context, id core.WorktreeID, mode core.PullMode) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	integrateArgs, err := pullIntegrateArgs(mode)
	if err != nil {
		return "", err
	}
	jobID := wt.Repo.Jobs().Start("pull", func(ctx context.Context, progress jobs.Progress) (any, error) {
		if err := fetchStep(ctx, wt, progress); err != nil {
			return nil, err
		}

		wt.Lock() // stash/integrate/pop all write this worktree's index/files
		defer wt.Unlock()

		before, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
		if err != nil {
			return nil, err
		}
		if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "push", "-m", stashMarker}}); err != nil {
			return nil, err
		}
		after, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
		if err != nil {
			return nil, err
		}

		if err := integrateStep(ctx, wt, integrateArgs); err != nil {
			return nil, err // stash left in place; the ordinary conflict-resolve flow handles it
		}
		if after == before {
			return nil, nil // nothing was stashed; no pop to do
		}
		ref, err := resolveStashRef(ctx, wt.Runner, wt.Path, after)
		if err != nil {
			return nil, err
		}
		_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "pop", "--end-of-options", ref}})
		return nil, err
	})
	return string(jobID), nil
}

// PopStash implements core.API.
func (c *Core) PopStash(ctx context.Context, id core.WorktreeID, sha string) error {
	return c.stashVerb(ctx, id, "pop", sha)
}

// DropStash implements core.API.
func (c *Core) DropStash(ctx context.Context, id core.WorktreeID, sha string) error {
	return c.stashVerb(ctx, id, "drop", sha)
}

// stashVerb runs git stash verb on the entry whose commit is sha, resolved
// to its current stash@{n} under the same write lock the verb runs under.
func (c *Core) stashVerb(ctx context.Context, id core.WorktreeID, verb, sha string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	ref, err := resolveStashRef(ctx, wt.Runner, wt.Path, sha)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", verb, "--end-of-options", ref}})
	return err
}

// stashListFormat pairs each stash list entry's commit with its current
// stash@{n}, newest first.
const stashListFormat = "%H%x1f%gd"

// resolveStashRef finds sha's current stash@{n} in dir's stash list. Code
// Internal if sha names no current entry: a stale caller-supplied ID
// (already popped or dropped elsewhere), not a git failure.
func resolveStashRef(ctx context.Context, r git.Runner, dir, sha string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"stash", "list", "--format=" + stashListFormat, "-z"},
	})
	if err != nil {
		return "", err
	}
	for _, rec := range nulRecords(result.Stdout) {
		commit, ref, ok := strings.Cut(rec, "\x1f")
		if ok && commit == sha {
			return ref, nil
		}
	}
	return "", core.Errorf(core.Internal, "stash %s is no longer in the stash list", sha)
}

// pullStash returns the commit of the newest stash entry stashMarker tags,
// "" if none. git stash list is newest-first.
func pullStash(ctx context.Context, r git.Runner, dir string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"stash", "list", "--format=%H%x1f%gs", "-z"},
	})
	if err != nil {
		return "", err
	}
	for _, rec := range nulRecords(result.Stdout) {
		sha, subject, ok := strings.Cut(rec, "\x1f")
		if ok && strings.Contains(subject, stashMarker) {
			return sha, nil
		}
	}
	return "", nil
}

// Push implements core.API.
func (c *Core) Push(ctx context.Context, id core.WorktreeID, remote string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	args := []string{"push", "--porcelain", "--progress"}
	if remote != "" {
		args = append(args, "--set-upstream", remote, "HEAD") // tracks a same-named remote branch
	}
	return startPushJob(wt, "push", args), nil
}

// ForcePush implements core.API.
func (c *Core) ForcePush(ctx context.Context, id core.WorktreeID) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	// Bare --force-with-lease, never =<ref>:<expect>, which would silently
	// disable --force-if-includes. Plain --force is never offered.
	args := []string{"push", "--porcelain", "--progress", "--force-with-lease", "--force-if-includes"}
	return startPushJob(wt, "force-push", args), nil
}

// UpdateBranch implements core.API.
func (c *Core) UpdateBranch(ctx context.Context, id core.WorktreeID, branch string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	sha, remote, upstreamRef, checkedOutIn, err := c.branchUpstreamInfo(ctx, wt.Runner, wt.Path, branch)
	if err != nil {
		return "", err
	}
	if upstreamRef == "" {
		return "", core.Errorf(core.NoUpstream, "%q has no upstream configured.", branch)
	}

	jobID := wt.Repo.Jobs().StartForBranch("update-branch", branch, func(ctx context.Context, progress jobs.Progress) (any, error) {
		toDir, toRunner := wt.Path, wt.Runner
		if checkedOutIn == "" {
			if err := c.fetchIntoLocalBranch(ctx, wt, remote, upstreamRef, branch, progress); err != nil {
				return nil, err
			}
		} else {
			other, err := c.sessions.Open(ctx, core.RepoPath(checkedOutIn))
			if err != nil {
				return nil, err
			}
			if err := c.fetchAndMergeElsewhere(ctx, other, remote, upstreamRef, branch, progress); err != nil {
				return nil, err
			}
			toDir, toRunner = other.Path, other.Runner
		}

		newSHA, err := revParse(ctx, toRunner, toDir, branch)
		if err != nil {
			return nil, err
		}
		commits := 0
		if newSHA != sha {
			commits, err = revListCount(ctx, toRunner, toDir, sha+".."+newSHA)
			if err != nil {
				return nil, err
			}
		}
		outcome := "upToDate"
		if commits > 0 {
			outcome = "updated"
		}
		return core.UpdateBranchResult{Outcome: outcome, From: sha, To: newSHA, Commits: commits}, nil
	})
	return string(jobID), nil
}

// fetchIntoLocalBranch is UpdateBranch's case where branch is checked out
// nowhere: fetched straight into it. Fast-forward-only by git's own
// refspec rule; a diverged refusal is reclassified NotFastForward, anything
// else stays plain GitFailed.
func (c *Core) fetchIntoLocalBranch(ctx context.Context, wt *session.Worktree, remote, upstreamRef, branch string, progress jobs.Progress) error {
	args := []string{"fetch", "--progress", remote, upstreamRef + ":refs/heads/" + branch}
	result, err := runWithProgress(ctx, wt.Runner, wt.Path, args, progress)
	return classifyDivergedFetch(remapAuthFailed(err, result), result, branch)
}

// fetchAndMergeElsewhere is UpdateBranch's case where branch is checked out
// in other, a different worktree: refused early if other is dirty — no
// network spent on a call already known to be refused. Otherwise fetches
// only the shared remote-tracking ref (no destination refspec, so the
// checked-out-branch fetch refusal never applies here) and merges --ff-only
// under other's own write queue, updating its files too.
func (c *Core) fetchAndMergeElsewhere(ctx context.Context, other *session.Worktree, remote, upstreamRef, branch string, progress jobs.Progress) error {
	if c.isDirty(ctx, other.Path) {
		return core.Errorf(core.DirtyWorktree, "%q is checked out in %s, which has uncommitted changes.", branch, other.Path)
	}
	result, err := runWithProgress(ctx, other.Runner, other.Path, []string{"fetch", "--progress", remote, upstreamRef}, progress)
	if err := remapAuthFailed(err, result); err != nil {
		return err
	}

	other.Lock()
	defer other.Unlock()
	_, err = other.Runner.Run(ctx, git.Call{Dir: other.Path, Args: []string{"merge", "--ff-only", "@{upstream}"}})
	return err // a non-nil err here is a race: something changed between the dirty check and now
}

// branchUpstreamFormat reads one branch's SHA, its upstream's remote ("."
// for a local upstream) and ref on that remote, and the worktree it is
// checked out in.
const branchUpstreamFormat = "%(objectname)%1f%(upstream:remotename)%1f%(upstream:remoteref)%1f%(worktreepath)%00"

// branchUpstreamInfo reads branch's SHA, upstream remote and ref (both ""
// if none configured), and the worktree it is checked out in ("" if none),
// in one read-only for-each-ref call.
func (c *Core) branchUpstreamInfo(ctx context.Context, r git.Runner, dir, branch string) (sha, remote, upstreamRef, checkedOutIn string, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"for-each-ref", "--format=" + branchUpstreamFormat, "refs/heads/" + branch},
	})
	if err != nil {
		return "", "", "", "", err
	}
	records := nulRecords(result.Stdout)
	if len(records) == 0 {
		return "", "", "", "", core.Errorf(core.Internal, "branch %q not found", branch)
	}
	f := strings.Split(records[0], "\x1f")
	return f[0], f[1], f[2], f[3], nil
}

// revParse resolves ref to its SHA, read-only.
func revParse(ctx context.Context, r git.Runner, dir, ref string) (string, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: []string{"rev-parse", "--verify", "--end-of-options", ref}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// revListCount counts the commits in rangeExpr (e.g. "<old>..<new>"), read-only.
func revListCount(ctx context.Context, r git.Runner, dir, rangeExpr string) (int, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: []string{"rev-list", "--count", rangeExpr}})
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(result.Stdout)))
	if err != nil {
		return 0, core.Errorf(core.Internal, "parsing rev-list --count output: %v", err)
	}
	return n, nil
}

// startPushJob starts kind ("push" or "force-push") running args as a job,
// classifying a rejection from --porcelain's summary line on stdout and a
// failed sign-in from stderr.
func startPushJob(wt *session.Worktree, kind string, args []string) string {
	jobID := wt.Repo.Jobs().Start(kind, func(ctx context.Context, progress jobs.Progress) (any, error) {
		result, err := runWithProgress(ctx, wt.Runner, wt.Path, args, progress)
		return nil, classifyPushRejection(remapAuthFailed(err, result), result)
	})
	return string(jobID)
}

// pullIntegrateArgs maps mode to its git call. An unknown mode is a caller
// bug: Code Internal.
func pullIntegrateArgs(mode core.PullMode) ([]string, error) {
	switch mode {
	case core.PullMerge:
		return []string{"merge", "--no-edit", "@{upstream}"}, nil
	case core.PullRebase:
		return []string{"rebase", "@{upstream}"}, nil
	default:
		return nil, core.Errorf(core.Internal, "unknown pull mode %q", mode)
	}
}

// pushRejectPrefix is --porcelain's own rejected-ref line prefix, always on
// stdout.
const pushRejectPrefix = "!\t"

// ffReasons pairs each porcelain rejection reason meaning the remote has
// commits not yet fetched and integrated locally — classified
// NotFastForward — with the sentence to show the user. Every other
// explained rejection is PushRejected.
var ffReasons = map[string]string{
	"fetch first":                       "The remote has commits you don't have yet (fetch first). Pull first, or force push.",
	"non-fast-forward":                  "Your branch isn't a fast-forward of the remote (non-fast-forward). Pull first, or force push.",
	"stale info":                        "The remote moved since your last fetch (stale info). Pull first, or force push.",
	"remote ref updated since checkout": "The remote has commits you fetched but haven't integrated (remote ref updated since checkout). Pull first, or force push.",
}

// classifyPushRejection reads a push's git.Result after remapAuthFailed (an
// auth failure never reaches the per-ref summary line).
func classifyPushRejection(err error, result git.Result) error {
	ce := gitFailure(err)
	if ce == nil {
		return err
	}
	reason, found := rejectReason(result.Stdout)
	if !found {
		return err // refused before any per-ref line printed (e.g. no upstream at all) — plain GitFailed
	}
	if msg, ok := ffReasons[reason]; ok {
		ce.Code = core.NotFastForward
		ce.Message = msg
	} else {
		ce.Code = core.PushRejected
		ce.Message = fmt.Sprintf("The remote refused the push (%s).", reason)
	}
	return err
}

// nonFastForwardFetch marks a fetch into a local branch refused as
// non-fast-forward. fetch has no --porcelain form, so this matches stderr.
const nonFastForwardFetch = "(non-fast-forward)"

// classifyDivergedFetch reclassifies a diverged UpdateBranch fetch as
// NotFastForward. Any other GitFailed refusal is left unchanged.
func classifyDivergedFetch(err error, result git.Result, branch string) error {
	if ce := gitFailure(err); ce != nil && bytes.Contains(result.Stderr, []byte(nonFastForwardFetch)) {
		ce.Code = core.NotFastForward
		ce.Message = fmt.Sprintf("%q has diverged from its upstream and can't be fast-forwarded.", branch)
	}
	return err
}

// rejectReason extracts <reason> from --porcelain's own
// "!\t<from>:<to>\t[rejected] (<reason>)" line.
func rejectReason(stdout []byte) (string, bool) {
	for _, line := range strings.Split(string(stdout), "\n") {
		if !strings.HasPrefix(line, pushRejectPrefix) {
			continue
		}
		open, close := strings.LastIndex(line, "("), strings.LastIndex(line, ")")
		if open >= 0 && close > open {
			return line[open+1 : close], true
		}
	}
	return "", false
}

// authFailurePrefixes are the two known git stderr shapes for a failed
// sign-in — exit 128, shared with every unrelated fatal, so message-prefix
// matching is unavoidable.
var authFailurePrefixes = []string{"fatal: could not read Username for", "fatal: Authentication failed for"}

// remapAuthFailed classifies a failed sign-in. Its message names no URL or
// credential; git's own stderr stays in GitStderr.
func remapAuthFailed(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && containsAny(string(result.Stderr), authFailurePrefixes) {
		ce.Code = core.AuthFailed
		ce.Message = "Could not sign in to the remote. Check your git credential setup (credential helper, SSH key, or cached login) outside the app — nebegit never asks for, stores, or sees credentials."
	}
	return err
}
