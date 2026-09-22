package core

// ChangedFiles groups a worktree's current changes (FR-CHG-01). A file
// changed both ways (git's own "MM"-shaped status line) appears once in
// Staged and once in Unstaged.
type ChangedFiles struct {
	Staged     []ChangedFile `json:"staged"`
	Unstaged   []ChangedFile `json:"unstaged"`
	Untracked  []ChangedFile `json:"untracked"`
	Conflicted []ChangedFile `json:"conflicted"`
}
