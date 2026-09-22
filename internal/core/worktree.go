package core

// Worktree is one row of ListWorktrees/AddWorktree's result (FR-WT-01).
type Worktree struct {
	Path     string `json:"path"`
	Branch   string `json:"branch,omitempty"` // "" when Detached
	Detached bool   `json:"detached"`
	SHA      string `json:"sha"`
	Main     bool   `json:"main"` // git's own "main worktree listed first" (Research used)
	Locked   bool   `json:"locked"`
	Missing  bool   `json:"missing"` // git's own "prunable" (Research used); Dirty is always false when true — its directory is gone, nothing to check
	Dirty    bool   `json:"dirty"`
}

// BranchSummary is one row of ListBranches' result.
type BranchSummary struct {
	Name         string `json:"name"`
	CheckedOutIn string `json:"checkedOutIn,omitempty"` // a worktree's path, "" if none (mirrors Branch.CheckedOutIn, spec 11)
}
