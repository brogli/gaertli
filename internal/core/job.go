package core

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
