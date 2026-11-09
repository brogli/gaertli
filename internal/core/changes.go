package core

import "context"

// Changes is the worktree's uncommitted work: its changes by state, staging
// and discarding files, hunks and lines, committing, the message draft, and a
// working-tree diff.
type Changes interface {
	// Status lists wt's uncommitted changes: staged, unstaged, untracked,
	// conflicted. A file changed both ways appears in Staged and Unstaged.
	Status(ctx context.Context, wt WorktreeID) (ChangedFiles, error)

	// StageFile (git add) and UnstageFile (git restore --staged) move one
	// path between the working tree and the index. Refusals are GitFailed.
	StageFile(ctx context.Context, wt WorktreeID, path string) error
	UnstageFile(ctx context.Context, wt WorktreeID, path string) error

	// StageAll (git add -A) and UnstageAll (git restore --staged -- .) act
	// on every changed file.
	StageAll(ctx context.Context, wt WorktreeID) error
	UnstageAll(ctx context.Context, wt WorktreeID) error

	// StageLines stages (or, if reverse, unstages) the changed lines sel
	// names in a fresh diff of path: the unstaged diff, or the staged diff
	// if reverse. If that diff's sides no longer match sel.Fingerprint,
	// path changed since it was shown: Code Internal, nothing written. A
	// selected line the fresh diff shows unchanged is skipped (as for a
	// selection from git diff -w's hunks); a selection with no changed line
	// is Code Internal too. Other refusals are GitFailed.
	StageLines(ctx context.Context, wt WorktreeID, path string, sel LineSelection, reverse bool) error

	// DiscardFile discards path's working-tree changes: git clean for an
	// untracked file or directory, else git restore to the index content.
	// The UI confirms first.
	DiscardFile(ctx context.Context, wt WorktreeID, path string, untracked bool) error

	// DiscardLines discards sel's lines of path's unstaged diff. Same
	// staleness rule as StageLines.
	DiscardLines(ctx context.Context, wt WorktreeID, path string, sel LineSelection) error

	// Commit commits the staged changes, or amends the last commit. message
	// goes over stdin (-F -), so it survives byte for byte and no editor
	// opens. Hooks run unmodified; a failing hook's output lands in the
	// error's GitStderr. Git's own refusals (nothing staged, empty message,
	// amend on an unborn HEAD) are GitFailed. Success clears the message
	// draft.
	Commit(ctx context.Context, wt WorktreeID, message string, amend bool) error

	// SetMessageDraft replaces wt's commit message draft, kept per worktree
	// session until Commit succeeds. Other windows on wt see it on their
	// next Summary; it is not pushed live.
	SetMessageDraft(ctx context.Context, wt WorktreeID, message string) error

	// DiffWorking returns path's two sides outside a commit: HEAD vs. the
	// index if staged, else the index (nothing for an untracked path) vs.
	// the working-tree file. Same rules as DiffFile. oldPath is a staged
	// rename's source.
	DiffWorking(ctx context.Context, wt WorktreeID, path, oldPath string, staged bool, opts DiffOptions) (DiffFile, error)

	// UncommittedCount is the hard-reset dialog's number: tracked paths with
	// a staged, unstaged, or unmerged change. Untracked files don't count; a
	// hard reset keeps them.
	UncommittedCount(ctx context.Context, wt WorktreeID) (int, error)
}

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
