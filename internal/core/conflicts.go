package core

import "context"

// Conflicts resolves the files a stopped operation left conflicted, and keeps
// the merge drafts.
type Conflicts interface {
	// Conflict reads path's conflict for the merge editor or the choice panel:
	// its kind, the operation and sides' labels, and for Kind text the three
	// sides, the working file as the result, its changes, its line ending and a
	// fingerprint. All text has LF line breaks. A path with no unmerged entry is
	// GitFailed (key notConflicted); a text conflict whose working file is gone
	// is FileNotFound.
	Conflict(ctx context.Context, wt WorktreeID, path string) (ConflictFile, error)

	// SaveConflict writes text (LF line breaks) to path's working file in its
	// line-ending convention and, if markResolved, stages it (git add). The
	// file must still match fingerprint (GitFailed, key fileChangedSinceOpened,
	// nothing written); markResolved with a conflict marker line left is
	// GitFailed (key conflictMarkersLeft), nothing written. Returns the new
	// fingerprint.
	SaveConflict(ctx context.Context, wt WorktreeID, path, text, fingerprint string, markResolved bool) (string, error)

	// ResolveConflict resolves path to one side or deletes it: current
	// (checkout --ours + add), incoming (checkout --theirs + add), or delete
	// (git rm). Current is always the side checked out (stage 2), also in a
	// rebase. Git's refusals (a side without a version) are GitFailed.
	ResolveConflict(ctx context.Context, wt WorktreeID, path string, choice ConflictChoice) error

	// ReopenConflict turns path, resolved while the operation is still
	// stopped, back into a conflict: its stages from git's resolve-undo record,
	// and for a file with both sides the working file as git merged it (its
	// markers). No operation in progress is GitFailed (key noOperation); no
	// record is GitFailed (key notReopenable).
	ReopenConflict(ctx context.Context, wt WorktreeID, path string) error

	// MergeDraft returns the unsaved merge result kept for path, nil if none.
	// Drafts are kept per worktree session, in memory only. A draft of a path
	// that is no longer conflicted, or was made for an earlier conflict in
	// path, is dropped, and nil returned.
	MergeDraft(ctx context.Context, wt WorktreeID, path string) (*MergeDraft, error)

	// SetMergeDraft keeps draft as path's unsaved merge result; nil clears it.
	// A draft whose ConflictID is not that of path's conflict now is not kept.
	SetMergeDraft(ctx context.Context, wt WorktreeID, path string, draft *MergeDraft) error
}

// ConflictFile is one conflicted file as the merge editor or the choice
// panel needs it.
type ConflictFile struct {
	Path       string        `json:"path"`
	Kind       ConflictKind  `json:"kind"`
	Operation  OperationKind `json:"operation"` // "" for a conflict no operation records (a stash pop)
	Base       ConflictSide  `json:"base"`
	Current    ConflictSide  `json:"current"`          // stage 2, the side checked out
	Incoming   ConflictSide  `json:"incoming"`         // stage 3, the side brought in
	Result     string        `json:"result,omitempty"` // Kind text: the working file
	MarkerSize int           `json:"markerSize,omitempty"`
	Changes    []MergeChange `json:"changes,omitempty"` // Kind text, in Result order
	// Kind text: where each side differs from the base, in the side's lines.
	CurrentChanges  []SideChange `json:"currentChanges,omitempty"`
	IncomingChanges []SideChange `json:"incomingChanges,omitempty"`
	Fingerprint     string       `json:"fingerprint,omitempty"`
	// ConflictID identifies the conflict as read: a later conflict in the
	// same path has another.
	ConflictID string `json:"conflictId,omitempty"`
}

// ConflictSide is one side's version and what it comes from.
type ConflictSide struct {
	Present bool   `json:"present"`           // the side has a version of the file
	Content string `json:"content,omitempty"` // Kind text only
	SHA     SHA    `json:"sha,omitempty"`     // the commit the side comes from; "" for base or unknown
	Ref     string `json:"ref,omitempty"`     // short name of a ref at SHA, "" if none
	Subject string `json:"subject,omitempty"` // SHA's subject, for a side that is one commit
}

// MergeChange is one change of a conflicted file: a run of lines where the
// two sides differ, and where its text sits in Result and in each side, as
// DiffHunk numbers lines: 1-based Start; with Lines 0, Start is the line
// after which the change would be. A conflict is a marker block of Result.
// Any other change is applied in Result already; its Result range holds the
// lines of the changed side. Added: the change added lines and replaced none
// of the base's.
type MergeChange struct {
	Kind     MergeChangeKind `json:"kind"`
	Added    bool            `json:"added,omitempty"`
	Result   LineRange       `json:"result"`
	Base     LineRange       `json:"base"`
	Current  LineRange       `json:"current"`
	Incoming LineRange       `json:"incoming"`
}

// MergeChangeKind says who made a MergeChange.
type MergeChangeKind string

const (
	ConflictChange     MergeChangeKind = "conflict" // both sides changed the lines differently
	CurrentOnlyChange  MergeChangeKind = "current"  // only the current side changed them
	IncomingOnlyChange MergeChangeKind = "incoming" // only the incoming side changed them
)

// SideChange is a run of a side's lines that differ from the base: lines the
// side added (Added) or changed.
type SideChange struct {
	Lines LineRange `json:"lines"`
	Added bool      `json:"added"`
}

// MergeDraft is a merge result with unsaved edits: its text (LF line
// breaks), its changes with their states, the fingerprint of
// the working file it was read from and the ConflictID of the conflict it was
// made for.
type MergeDraft struct {
	Text        string        `json:"text"`
	Fingerprint string        `json:"fingerprint"`
	ConflictID  string        `json:"conflictId"`
	Changes     []DraftChange `json:"changes"`
}

// DraftChange is one change (a conflict or a one-side change) of a MergeDraft's text and whether the user
// resolved it.
type DraftChange struct {
	Range    LineRange `json:"range"`
	Resolved bool      `json:"resolved"`
}

// LineRange is a run of Lines lines from line Start.
type LineRange struct {
	Start int `json:"start"`
	Lines int `json:"lines"`
}

// ConflictKind says how a conflicted file can be resolved.
type ConflictKind string

const (
	ConflictText      ConflictKind = "text"
	ConflictNotUTF8   ConflictKind = "notUtf8"
	ConflictBinary    ConflictKind = "binary"
	ConflictSymlink   ConflictKind = "symlink"
	ConflictDeleted   ConflictKind = "deleted"   // one side deleted the file
	ConflictRename    ConflictKind = "rename"    // renamed differently on the two sides
	ConflictSubmodule ConflictKind = "submodule" // a submodule pointer
)

// ConflictChoice is the side ResolveConflict takes, or the deletion.
type ConflictChoice string

const (
	ChooseCurrent  ConflictChoice = "current"
	ChooseIncoming ConflictChoice = "incoming"
	ChooseDelete   ConflictChoice = "delete"
)
