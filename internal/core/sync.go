package core

// PullMode is how a pull integrates the fetched upstream.
type PullMode string

const (
	PullMerge  PullMode = "merge"
	PullRebase PullMode = "rebase"
)

// UpdateBranchResult is UpdateBranch's JobInfo.Result, set only when the
// job ends done.
type UpdateBranchResult struct {
	Outcome string `json:"outcome"` // "updated" | "upToDate"
	From    string `json:"from"`    // branch's SHA before
	To      string `json:"to"`      // branch's SHA after
	Commits int    `json:"commits"` // commits reachable from To not From; 0 when upToDate
}
