package core

// ChangedFiles groups a worktree's current changes. A file changed both
// ways (git's "MM"-shaped status line) appears once in Staged and once in
// Unstaged. Resolved lists, while an operation is in progress, the paths
// resolved since it started that ReopenConflict can turn back into conflicts.
type ChangedFiles struct {
	Staged     []ChangedFile `json:"staged"`
	Unstaged   []ChangedFile `json:"unstaged"`
	Untracked  []ChangedFile `json:"untracked"`
	Conflicted []ChangedFile `json:"conflicted"`
	Resolved   []string      `json:"resolved"`
}

// LineSelection names the changed lines of one path's diff to stage,
// unstage or discard, by each line's 1-based number on its side as the
// diff was shown: Old for lines removed from its old content, New for lines
// added to its new content. Fingerprint is the shown DiffFile's.
type LineSelection struct {
	Old         []int  `json:"old"`
	New         []int  `json:"new"`
	Fingerprint string `json:"fingerprint"`
}
