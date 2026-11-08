package core

import "context"

// Sync exchanges commits with remotes as jobs: fetch, pull, push, force
// push, updating a branch; and the jobs themselves.
type Sync interface {
	// Fetch fetches remote, or every remote if "", pruning gone branches,
	// as a job outside the write queues: it touches only refs and objects.
	// Returns the job id; the result arrives via "job:update".
	Fetch(ctx context.Context, wt WorktreeID, remote string) (string, error)

	// Pull fetches the current branch's upstream without a lock, then
	// merges or rebases under wt's write lock, as one job. The step equals
	// git pull --no-rebase or --rebase with the user's config: the merge
	// follows pull.ff, the rebase uses the fork point. Detached or without
	// an upstream, git's refusal surfaces as-is. A stop mid-way is Code
	// Conflicted, left for the resolve flow.
	Pull(ctx context.Context, wt WorktreeID, mode PullMode) (string, error)

	// PullWithStash is Pull with local changes in the way: after fetching,
	// it stashes them (tagged, so Summary.WaitingStash finds the entry),
	// integrates, and pops that exact entry only if the integrate step ends
	// cleanly. An integrate failure keeps the stash and is returned as-is;
	// a conflicting pop is GitFailed.
	PullWithStash(ctx context.Context, wt WorktreeID, mode PullMode) (string, error)

	// Push pushes branch as a job outside the write queues: plainly if
	// remote is "" (needs an upstream; branch is ignored, HEAD's branch
	// pushes), else refs/heads/branch with --set-upstream <remote>. A
	// rejection is classified from --porcelain's summary line:
	// NotFastForward if the remote has unfetched commits, else
	// PushRejected. A first push's NotFastForward (remote given) carries
	// its own message, naming neither pull nor force push, since neither
	// applies without an upstream. GitStderr carries git's explanation.
	Push(ctx context.Context, wt WorktreeID, remote, branch string) (string, error)

	// PushTarget is where a plain push of the current branch would go and
	// whether it would ask first. Remote is %(push:remotename), else the
	// only remote, else origin if it exists; "" means none applies and the
	// UI must ask which remote. AutoSetUpstream is push.autoSetupRemote.
	// Branch is HEAD's branch; HasUpstream is read fresh from git (unlike a
	// cached refs store), for a caller that must still push the named
	// branch after a config-only change elsewhere. A detached HEAD or a bad
	// push.autoSetupRemote value is Code GitFailed with git's stderr.
	PushTarget(ctx context.Context, wt WorktreeID) (PushTarget, error)

	// ForcePush pushes with --force-with-lease --force-if-includes, never
	// plain --force. Protected-branch warnings are the UI's job. Rejections
	// are classified like Push's.
	ForcePush(ctx context.Context, wt WorktreeID) (string, error)

	// UpdateBranch fast-forwards branch from its upstream as a job, fetching
	// straight into it. Refused before the job starts: no upstream configured
	// (Code NoUpstream), checked out in a worktree (Code GitFailed, key
	// branchCheckedOutElsewhere), or being rebased or bisected in a worktree
	// (Code GitFailed, key branchRebasingElsewhere or branchBisectingElsewhere).
	// A diverged branch is NotFastForward. The job's Result is an
	// UpdateBranchResult.
	UpdateBranch(ctx context.Context, wt WorktreeID, branch string) (string, error)

	// Jobs lists wt's repo's running jobs plus a bounded history
	// of finished ones. Updates arrive as "job:update" events.
	Jobs(ctx context.Context, wt WorktreeID) ([]JobInfo, error)

	// CancelJob cancels job id, killing its git process. No-op for an
	// unknown or finished id.
	CancelJob(ctx context.Context, wt WorktreeID, id string) error
}

// PushTarget is PushTarget's result.
type PushTarget struct {
	Remote          string `json:"remote"`          // "" means the UI must ask which remote
	AutoSetUpstream bool   `json:"autoSetUpstream"` // push.autoSetupRemote
	Branch          string `json:"branch"`          // HEAD's branch, to push even if HEAD moves before the caller acts
	HasUpstream     bool   `json:"hasUpstream"`     // read fresh from git, unlike a cached refs store
}

// PullMode is how a pull integrates the fetched upstream.
type PullMode string

const (
	PullMerge  PullMode = "merge"
	PullRebase PullMode = "rebase"
)

// UpdateBranchResult is UpdateBranch's JobInfo.Result, set only when the
// job ends done.
type UpdateBranchResult struct {
	Outcome UpdateOutcome `json:"outcome"`
	From    string        `json:"from"`    // branch's SHA before
	To      string        `json:"to"`      // branch's SHA after
	Commits int           `json:"commits"` // commits reachable from To not From; 0 when BranchUpToDate
}

// UpdateOutcome is whether UpdateBranch moved the branch.
type UpdateOutcome string

const (
	BranchUpdated  UpdateOutcome = "updated"
	BranchUpToDate UpdateOutcome = "upToDate"
)

// JobInfo is one job's current state, as Jobs lists it and "job:update"
// carries it.
type JobInfo struct {
	ID       string              `json:"id"`
	Kind     JobKind             `json:"kind"`
	Branch   string              `json:"branch,omitempty"` // the branch an UpdateBranchJob acts on
	State    JobState            `json:"state"`
	Progress int                 `json:"progress"` // 0-100, or -1 while indeterminate
	Message  string              `json:"message"`
	Error    *Error              `json:"error,omitempty"`  // set only when State is JobFailed
	Result   *UpdateBranchResult `json:"result,omitempty"` // an UpdateBranchJob's, set only when State is JobDone
}

// JobKind is what a job does.
type JobKind string

const (
	FetchJob        JobKind = "fetch"
	PullJob         JobKind = "pull"
	PushJob         JobKind = "push"
	ForcePushJob    JobKind = "force-push"
	UpdateBranchJob JobKind = "update-branch"
)

// JobState is a job's lifecycle stage.
type JobState string

const (
	JobRunning   JobState = "running"
	JobDone      JobState = "done"
	JobFailed    JobState = "failed"
	JobCancelled JobState = "cancelled"
)
