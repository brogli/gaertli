package core

// JobInfo is one job's current state, as Jobs lists it and "job:update"
// carries it.
type JobInfo struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	State     string `json:"state"`    // "queued" | "running" | "done" | "failed" | "cancelled"
	Progress  int    `json:"progress"` // 0-100, or -1 while indeterminate
	Message   string `json:"message"`
	StartedAt string `json:"startedAt"` // RFC3339Nano
	EndedAt   string `json:"endedAt,omitempty"`
	Error     *Error `json:"error,omitempty"`  // set only when State is "failed"
	Result    any    `json:"result,omitempty"` // job-kind-specific payload, set only when State is "done"
}
