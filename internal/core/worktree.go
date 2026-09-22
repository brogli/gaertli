package core

// Worktree is one row of ListWorktrees/AddWorktree's result.
type Worktree struct {
	Path     string `json:"path"`
	Branch   string `json:"branch,omitempty"` // "" when Detached
	Detached bool   `json:"detached"`
	SHA      string `json:"sha"`
	Main     bool   `json:"main"` // git lists the main worktree first
	Locked   bool   `json:"locked"`
	Missing  bool   `json:"missing"` // git's "prunable"; Dirty is always false then, the directory is gone
	Dirty    bool   `json:"dirty"`
}

// BranchSummary is one row of ListBranches' result.
type BranchSummary struct {
	Name         string `json:"name"`
	CheckedOutIn string `json:"checkedOutIn,omitempty"` // a worktree's path, "" if none (mirrors Branch.CheckedOutIn)
}
