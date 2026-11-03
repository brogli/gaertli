package core

// Worktree is one row of ListWorktrees/AddWorktree's result.
type Worktree struct {
	Path    string `json:"path"`
	Branch  string `json:"branch,omitempty"` // "" when detached
	SHA     string `json:"sha"`
	Main    bool   `json:"main"` // git lists the main worktree first
	Locked  bool   `json:"locked"`
	Missing bool   `json:"missing"` // git's "prunable"; Dirty is always false then, the directory is gone
	Dirty   bool   `json:"dirty"`
	Bare    bool   `json:"bare,omitempty"` // git's "bare": a bare repo's own entry; no files, so Dirty is always false
}
