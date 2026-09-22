package core

// JobInfo is one job's current state, matching components.md's "job:update
// {id, state, progress, message}" (plus Kind/StartedAt/EndedAt/Error, which
// that shorthand omits the same way "repo:changed {scope}" omits the Event
// envelope's own Repo/Worktree fields).
type JobInfo struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	State     string `json:"state"`    // "queued" | "running" | "done" | "failed" | "cancelled"
	Progress  int    `json:"progress"` // 0-100, or -1 while indeterminate
	Message   string `json:"message"`
	StartedAt string `json:"startedAt"` // RFC3339Nano
	EndedAt   string `json:"endedAt,omitempty"`
	Error     *Error `json:"error,omitempty"` // set only when State is "failed"
}
