package core

// ConflictFile is one conflicted file as the merge editor or the choice
// panel needs it.
type ConflictFile struct {
	Path       string           `json:"path"`
	Kind       ConflictKind     `json:"kind"`
	Operation  Operation        `json:"operation"` // "" for a conflict no operation records (a stash pop)
	Base       ConflictSide     `json:"base"`
	Current    ConflictSide     `json:"current"`          // stage 2, the side checked out
	Incoming   ConflictSide     `json:"incoming"`         // stage 3, the side brought in
	Result     string           `json:"result,omitempty"` // Kind text: the working file
	MarkerSize int              `json:"markerSize,omitempty"`
	Regions    []ConflictRegion `json:"regions,omitempty"` // Kind text, in Result order
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
	SHA     string `json:"sha,omitempty"`     // the commit the side comes from; "" for base or unknown
	Ref     string `json:"ref,omitempty"`     // short name of a ref at SHA, "" if none
	Subject string `json:"subject,omitempty"` // SHA's subject, for a side that is one commit
}

// ConflictRegion is one marker block of Result and where its text sits in
// each side, as DiffHunk numbers lines: 1-based Start; with Lines 0, Start
// is the line after which the region would be.
type ConflictRegion struct {
	Result   LineRange `json:"result"`
	Base     LineRange `json:"base"`
	Current  LineRange `json:"current"`
	Incoming LineRange `json:"incoming"`
}

// SideChange is a run of a side's lines that differ from the base: lines the
// side added (Added) or changed.
type SideChange struct {
	Lines LineRange `json:"lines"`
	Added bool      `json:"added"`
}

// MergeDraft is a merge result with unsaved edits: its text (LF line
// breaks), the conflict regions in it with their states, the fingerprint of
// the working file it was read from and the ConflictID of the conflict it was
// made for.
type MergeDraft struct {
	Text        string        `json:"text"`
	Fingerprint string        `json:"fingerprint"`
	ConflictID  string        `json:"conflictId"`
	Regions     []DraftRegion `json:"regions"`
}

// DraftRegion is one conflict region of a MergeDraft's text.
type DraftRegion struct {
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
