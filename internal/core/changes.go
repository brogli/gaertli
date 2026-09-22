package core

// ChangedFiles groups a worktree's current changes. A file changed both
// ways (git's "MM"-shaped status line) appears once in Staged and once in
// Unstaged.
type ChangedFiles struct {
	Staged     []ChangedFile `json:"staged"`
	Unstaged   []ChangedFile `json:"unstaged"`
	Untracked  []ChangedFile `json:"untracked"`
	Conflicted []ChangedFile `json:"conflicted"`
}

// LineSelection names the changed lines of one path's diff to stage or
// unstage, by each line's 1-based number on the diff side it
// changed: Old for a line removed from the diff's old content, New for a
// line added to its new content. A number naming an unchanged (context)
// line, or no longer naming any changed line at all, matches nothing.
type LineSelection struct {
	Old []int `json:"old"`
	New []int `json:"new"`
}
