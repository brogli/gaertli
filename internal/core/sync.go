package core

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

// IntegrateResult is Merge's and Rebase's result.
type IntegrateResult struct {
	Outcome IntegrateOutcome `json:"outcome"`
}

// IntegrateOutcome is what Merge or Rebase did to the current branch.
type IntegrateOutcome string

const (
	IntegrateUpToDate    IntegrateOutcome = "upToDate"    // HEAD did not move
	IntegrateFastForward IntegrateOutcome = "fastForward" // Merge moved HEAD to the merged ref
	IntegrateMerged      IntegrateOutcome = "merged"      // Merge made a merge commit
	IntegrateRebased     IntegrateOutcome = "rebased"
)
