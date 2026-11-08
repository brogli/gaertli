package services

import (
	"bytes"
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/jobs"
	"github.com/brogli/gaertli/internal/session"
)

// remoteTimeout is git.Call's own safety-net Timeout for a remote call,
// generous on purpose: the job's own cancel through ctx is the real limit.
const remoteTimeout = time.Hour

// runRemote runs a --progress remote call in wt live, reporting every
// parsed line, and classifies a failed sign-in with remapAuthFailed. A line
// with no percent keeps the last known percent and updates only the
// message, so a status line between two percent phases never blanks the
// job's progress bar.
func runRemote(ctx context.Context, wt *session.Worktree, args []string, progress jobs.Progress) (git.Result, error) {
	percent := -1
	result, err := wt.Runner.StreamErr(ctx, git.Call{Dir: wt.Path, Args: args, Timeout: remoteTimeout}, func(line []byte) error {
		if p, message, ok := git.ParseProgressLine(line); ok {
			percent = p
			progress(percent, message)
		} else {
			progress(percent, string(line))
		}
		return nil
	})
	return result, remapAuthFailed(err, result)
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
	jobID := wt.Repo.Jobs().Start(ctx, core.FetchJob, func(ctx context.Context, progress jobs.Progress) (any, error) {
		_, err := runRemote(ctx, wt, args, progress)
		return nil, err
	})
	return string(jobID), nil
}

// Pull implements core.API.
func (c *Core) Pull(ctx context.Context, id core.WorktreeID, mode core.PullMode) (string, error) {
	return c.startPull(ctx, id, mode, integrateStep)
}

// PullWithStash implements core.API.
func (c *Core) PullWithStash(ctx context.Context, id core.WorktreeID, mode core.PullMode) (string, error) {
	return c.startPull(ctx, id, mode, integrateWithStash)
}

// startPull starts a pull job: a fetch without a lock, then integrate
// under wt's write lock.
func (c *Core) startPull(ctx context.Context, id core.WorktreeID, mode core.PullMode, integrate func(context.Context, *session.Worktree, []string) error) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	integrateArgs, err := pullIntegrateArgs(ctx, wt.Runner, wt.Path, mode)
	if err != nil {
		return "", err
	}
	jobID := wt.Repo.Jobs().Start(ctx, core.PullJob, func(ctx context.Context, progress jobs.Progress) (any, error) {
		if _, err := runRemote(ctx, wt, []string{"fetch", "--prune", "--progress"}, progress); err != nil {
			return nil, err
		}
		wt.Lock()
		defer wt.Unlock()
		return nil, integrate(ctx, wt, integrateArgs)
	})
	return string(jobID), nil
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

func containsAny(s string, subs []string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}

// stashSubjectPrefix starts the subject of every stash entry the app makes around an integrate,
// so Summary's WaitingStash tells it apart from a stash made outside the app. git prefixes the
// branch ("On main: "); the verb (pull, merge or rebase) follows.
const stashSubjectPrefix = "Gärtli: local changes before "

// integrateWithStash is integrateStep with local changes stashed around it.
func integrateWithStash(ctx context.Context, wt *session.Worktree, integrateArgs []string) error {
	return withStash(ctx, wt, "pull", func() error { return integrateStep(ctx, wt, integrateArgs) })
}

// withStash runs integrate with local changes stashed around it, the entry's subject naming verb.
// git stash push exits 0 without an entry when there is nothing to save, so refs/stash is compared
// before and after the push. The new entry is popped by its commit, never by a bare "stash pop"
// that could take someone else's. A failed integrate that left the worktree untouched gives the
// entry back; any other failure keeps it. Caller holds wt's write lock.
func withStash(ctx context.Context, wt *session.Worktree, verb string, integrate func() error) error {
	before, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
	if err != nil {
		return err
	}
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "push", "-m", stashSubjectPrefix + verb}}); err != nil {
		return err
	}
	after, err := verifyRev(ctx, wt.Runner, wt.Path, "refs/stash")
	if err != nil {
		return err
	}

	stashed := after != before
	var head string
	if stashed {
		if head, err = revParse(ctx, wt.Runner, wt.Path, "HEAD"); err != nil {
			return err
		}
	}
	if err := integrate(); err != nil {
		if stashed && untouched(ctx, wt, head) {
			_ = popStash(ctx, wt, after) // on failure the entry stays for the user
		}
		return err
	}
	if !stashed {
		return nil
	}
	return popStash(ctx, wt, after)
}

// untouched reports whether wt has no operation in progress, HEAD at head, and the index and
// tracked files as HEAD has them.
func untouched(ctx context.Context, wt *session.Worktree, head string) bool {
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

// popStash pops the stash entry whose commit is sha.
func popStash(ctx context.Context, wt *session.Worktree, sha string) error {
	ref, err := resolveStashRef(ctx, wt.Runner, wt.Path, sha)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"stash", "pop", "--end-of-options", ref}})
	return err
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

// waitingStash returns the commit of the newest stash entry the app made around an integrate, ""
// if none. git stash list is newest-first.
func waitingStash(ctx context.Context, r git.Runner, dir string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"stash", "list", "--format=%H%x1f%gs", "-z"},
	})
	if err != nil {
		return "", err
	}
	for _, rec := range nulRecords(result.Stdout) {
		sha, subject, ok := strings.Cut(rec, "\x1f")
		if ok && strings.Contains(subject, stashSubjectPrefix) {
			return sha, nil
		}
	}
	return "", nil
}

// Push implements core.API.
func (c *Core) Push(ctx context.Context, id core.WorktreeID, remote, branch string) (string, error) {
	args := []string{"push", "--porcelain", "--progress"}
	firstPush := remote != ""
	if firstPush {
		// The branch as named by the caller, not HEAD: HEAD may have moved
		// to another branch since the caller last asked (PushTarget or the
		// refs store) which branch to push.
		args = append(args, "--set-upstream", remote, "refs/heads/"+branch)
	}
	return c.startPushJob(ctx, id, core.PushJob, args, firstPush)
}

// ForcePush implements core.API.
func (c *Core) ForcePush(ctx context.Context, id core.WorktreeID) (string, error) {
	// Bare --force-with-lease, never =<ref>:<expect>, which would silently
	// disable --force-if-includes. Plain --force is never offered.
	args := []string{"push", "--porcelain", "--progress", "--force-with-lease", "--force-if-includes"}
	return c.startPushJob(ctx, id, core.ForcePushJob, args, false)
}

// startPushJob starts a push job of kind running args. firstPush marks a
// push that sets a new upstream, for classifyPushRejection.
func (c *Core) startPushJob(ctx context.Context, id core.WorktreeID, kind core.JobKind, args []string, firstPush bool) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	jobID := wt.Repo.Jobs().Start(ctx, kind, func(ctx context.Context, progress jobs.Progress) (any, error) {
		result, err := runRemote(ctx, wt, args, progress)
		return nil, classifyPushRejection(err, result, firstPush)
	})
	return string(jobID), nil
}

// PushTarget implements core.API.
func (c *Core) PushTarget(ctx context.Context, id core.WorktreeID) (core.PushTarget, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.PushTarget{}, err
	}
	ref, err := currentRef(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.PushTarget{}, err
	}
	remote, hasUpstream, err := pushRemoteAndUpstream(ctx, wt.Runner, wt.Path, ref)
	if err != nil {
		return core.PushTarget{}, err
	}
	autoSetUpstream, err := pushAutoSetupRemote(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.PushTarget{}, err
	}
	return core.PushTarget{
		Remote: remote, AutoSetUpstream: autoSetUpstream,
		Branch: strings.TrimPrefix(ref, "refs/heads/"), HasUpstream: hasUpstream,
	}, nil
}

// currentRef is HEAD's full ref name (e.g. "refs/heads/main"); a detached
// HEAD is git's own refusal (Code GitFailed).
func currentRef(ctx context.Context, r git.Runner, dir string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"symbolic-ref", "--quiet", "HEAD"},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// pushRemoteAndUpstream is the remote git would push ref to, and whether ref
// has an upstream configured. Remote is %(push:remotename)'s
// pushRemote/pushDefault/branch.remote, else fallbackPushRemote.
// hasUpstream is %(upstream) non-empty, read fresh so a config-only change
// (e.g. `git branch -u`) shows without waiting on a cached refs store.
func pushRemoteAndUpstream(ctx context.Context, r git.Runner, dir, ref string) (remote string, hasUpstream bool, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"for-each-ref", "--format=%(push:remotename)%1f%(upstream)", ref},
	})
	if err != nil {
		return "", false, err
	}
	remote, upstream, _ := strings.Cut(strings.TrimSuffix(string(result.Stdout), "\n"), "\x1f")
	if remote == "" {
		remote, err = fallbackPushRemote(ctx, r, dir)
	}
	return remote, upstream != "", err
}

// fallbackPushRemote is the sole remote, else origin if it exists, else "":
// the fallbacks %(push:remotename) does not apply itself.
func fallbackPushRemote(ctx context.Context, r git.Runner, dir string) (string, error) {
	remotes, err := remoteNames(ctx, r, dir)
	switch {
	case err != nil:
		return "", err
	case len(remotes) == 1:
		return remotes[0], nil
	case slices.Contains(remotes, "origin"):
		return "origin", nil
	}
	return "", nil
}

// pushAutoSetupRemote reads push.autoSetupRemote, false if unset. Unlike
// configValue's bool-or-str, --type=bool rejects a bad value as git's own
// GitFailed refusal instead of passing it through as a string.
func pushAutoSetupRemote(ctx context.Context, r git.Runner, dir string) (bool, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"config", "--type=bool", "--get", "push.autoSetupRemote"},
	})
	if err != nil && !quietFailure(err, result) {
		return false, err
	}
	return strings.TrimSpace(string(result.Stdout)) == "true", nil
}

// UpdateBranch implements core.API.
func (c *Core) UpdateBranch(ctx context.Context, id core.WorktreeID, branch string) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	b, err := branchUpstreamInfo(ctx, wt.Runner, wt.Path, branch)
	if err != nil {
		return "", err
	}
	if b.upstreamRef == "" {
		return "", core.ErrorfArgs(core.NoUpstream, "noUpstream", map[string]string{"branch": branch}, "%q has no upstream configured.", branch)
	}
	if b.checkedOutIn != "" {
		args := map[string]string{"branch": branch, "path": b.checkedOutIn}
		return "", core.ErrorfArgs(core.GitFailed, "branchCheckedOutElsewhere", args, "%q is checked out in %s. Pull it in that worktree.", branch, b.checkedOutIn)
	}
	uses, err := branchesInUse(ctx, wt.Runner, wt.Path, false)
	if err != nil {
		return "", err
	}
	if use, found := uses[branch]; found {
		return "", inUseRefusal(branch, use)
	}

	jobID := wt.Repo.Jobs().StartForBranch(ctx, core.UpdateBranchJob, branch, func(ctx context.Context, progress jobs.Progress) (any, error) {
		if err := fetchIntoLocalBranch(ctx, wt, b, progress); err != nil {
			return nil, err
		}
		return branchMoved(ctx, wt, b)
	})
	return string(jobID), nil
}

// branchMoved reads how far b moved from b.sha in wt.
func branchMoved(ctx context.Context, wt *session.Worktree, b branchUpstream) (any, error) {
	newSHA, err := revParse(ctx, wt.Runner, wt.Path, "refs/heads/"+b.name) // a tag of the same name would win over a bare name
	if err != nil {
		return nil, err
	}
	commits := 0
	if newSHA != b.sha {
		commits, err = revListCount(ctx, wt.Runner, wt.Path, b.sha+".."+newSHA)
		if err != nil {
			return nil, err
		}
	}
	outcome := core.BranchUpToDate
	if commits > 0 {
		outcome = core.BranchUpdated
	}
	return core.UpdateBranchResult{Outcome: outcome, From: b.sha, To: newSHA, Commits: commits}, nil
}

// fetchIntoLocalBranch is UpdateBranch's case where b is checked out
// nowhere: fetched straight into it. Fast-forward-only by git's own
// refspec rule; a diverged refusal is reclassified NotFastForward, anything
// else stays plain GitFailed.
func fetchIntoLocalBranch(ctx context.Context, wt *session.Worktree, b branchUpstream, progress jobs.Progress) error {
	args := []string{"fetch", "--progress", b.remote, b.upstreamRef + ":refs/heads/" + b.name}
	result, err := runRemote(ctx, wt, args, progress)
	return classifyDivergedFetch(err, result, b.name)
}

// branchUpstreamFormat reads one branch's SHA, its upstream's remote ("."
// for a local upstream) and ref on that remote, and the worktree it is
// checked out in.
const branchUpstreamFormat = "%(objectname)%1f%(upstream:remotename)%1f%(upstream:remoteref)%1f%(worktreepath)%00"

// branchUpstream is branch name's branchUpstreamFormat record: remote and
// upstreamRef are "" if no upstream is configured, checkedOutIn "" if the
// branch is checked out nowhere.
type branchUpstream struct {
	name, sha, remote, upstreamRef, checkedOutIn string
}

// branchUpstreamInfo reads branch's branchUpstream in one read-only
// for-each-ref call.
func branchUpstreamInfo(ctx context.Context, r git.Runner, dir, branch string) (branchUpstream, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"for-each-ref", "--format=" + branchUpstreamFormat, "refs/heads/" + branch},
	})
	if err != nil {
		return branchUpstream{}, err
	}
	records := nulRecords(result.Stdout)
	if len(records) == 0 {
		return branchUpstream{}, core.Errorf(core.Internal, "branch %q not found", branch)
	}
	f := strings.Split(records[0], "\x1f")
	return branchUpstream{name: branch, sha: f[0], remote: f[1], upstreamRef: f[2], checkedOutIn: f[3]}, nil
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

// pullIntegrateArgs maps mode to the merge or rebase call git pull
// --no-rebase or --rebase makes with dir's config. An unknown mode is a
// caller bug: Code Internal.
func pullIntegrateArgs(ctx context.Context, r git.Runner, dir string, mode core.PullMode) ([]string, error) {
	switch mode {
	case core.PullMerge:
		ffArgs, err := pullFFArgs(ctx, r, dir)
		if err != nil {
			return nil, err
		}
		return slices.Concat([]string{"merge", "--no-edit"}, ffArgs, []string{"@{upstream}"}), nil
	case core.PullRebase:
		// An explicit upstream turns the fork point off unless asked for; git
		// pull --rebase uses it to drop commits the upstream rewrote.
		return []string{"rebase", "--fork-point", "@{upstream}"}, nil
	default:
		return nil, core.Errorf(core.Internal, "unknown pull mode %q", mode)
	}
}

// pullFFArgs maps pull.ff to its merge flag. git merge reads merge.ff, not
// pull.ff, and an explicit --no-rebase pull merges even with pull.ff=only,
// so "only" and unset add no flag.
func pullFFArgs(ctx context.Context, r git.Runner, dir string) ([]string, error) {
	pullFF, err := configValue(ctx, r, dir, "pull.ff")
	switch {
	case err != nil:
		return nil, err
	case pullFF == "false":
		return []string{"--no-ff"}, nil
	case pullFF == "true":
		return []string{"--ff"}, nil
	default:
		return nil, nil
	}
}

// configValue reads key from dir's git config, a boolean spelled "true"
// or "false" whatever the file says; "" if unset.
func configValue(ctx context.Context, r git.Runner, dir, key string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"config", "--get", "--type=bool-or-str", key},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// pushRejectPrefix is --porcelain's own rejected-ref line prefix, always on
// stdout.
const pushRejectPrefix = "!\t"

// classifyPushRejection classifies a rejection from --porcelain's per-ref
// line: NotFastForward when the remote has commits not integrated locally,
// else PushRejected. firstPush marks a push that was setting a new upstream
// (no upstream existed yet): its NotFastForward carries its own message,
// since a first push has no fetched state to blame and no upstream to pull
// from or force-push over.
func classifyPushRejection(err error, result git.Result, firstPush bool) error {
	ce := gitFailure(err)
	if ce == nil {
		return err
	}
	reason, found := rejectReason(result.Stdout)
	if !found {
		return err // refused before any per-ref line: stays GitFailed
	}
	if firstPush {
		switch reason {
		case "fetch first", "non-fast-forward", "stale info", "remote ref updated since checkout":
			ce.Retag(core.NotFastForward, "pushFirstRejected", nil,
				"The remote already has a branch of that name with commits this branch lacks. Nothing was pushed.")
			return err
		}
	}
	switch reason {
	case "fetch first":
		ce.Retag(core.NotFastForward, "pushFetchFirst", nil, "The remote has commits you don't have yet (fetch first). Pull first, or force push.")
	case "non-fast-forward":
		ce.Retag(core.NotFastForward, "pushNotFastForward", nil, "Your branch isn't a fast-forward of the remote (non-fast-forward). Pull first, or force push.")
	case "stale info":
		ce.Retag(core.NotFastForward, "pushStaleInfo", nil, "The remote moved since your last fetch (stale info). Pull first, or force push.")
	case "remote ref updated since checkout":
		ce.Retag(core.NotFastForward, "pushRefUpdatedSinceCheckout", nil, "The remote has commits you fetched but haven't integrated (remote ref updated since checkout). Pull first, or force push.")
	default:
		ce.Retag(core.PushRejected, "pushRejected", map[string]string{"reason": reason}, "The remote refused the push (%s).", reason)
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
		retagDiverged(ce, branch)
	}
	return err
}

// retagDiverged reclassifies ce, an update of branch that git refused, as
// NotFastForward.
func retagDiverged(ce *core.Error, branch string) {
	ce.Retag(core.NotFastForward, "branchDiverged", map[string]string{"branch": branch}, "%q has diverged from its upstream and can't be fast-forwarded.", branch)
}

// rejectReason extracts <reason> from --porcelain's own
// "!\t<from>:<to>\t[rejected] (<reason>)" line.
func rejectReason(stdout []byte) (string, bool) {
	for line := range strings.SplitSeq(string(stdout), "\n") {
		if !strings.HasPrefix(line, pushRejectPrefix) {
			continue
		}
		start, end := strings.LastIndex(line, "("), strings.LastIndex(line, ")")
		if start >= 0 && end > start {
			return line[start+1 : end], true
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
		ce.Retag(core.AuthFailed, "authFailed", nil, "Could not sign in to the remote. Sign in once from a terminal (credential helper, SSH agent or cached login), then try again; Gärtli never asks for, stores or sees credentials.")
	}
	return err
}
