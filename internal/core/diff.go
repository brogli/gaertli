package core

// ChangedFile is one file touched by a commit, or one entry in a worktree's
// current changes.
type ChangedFile struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // rename/copy source
	// Status is "A" | "M" | "D" | "R" | "C" | "T" | "U" for a commit's own
	// file; "?" for an untracked entry, or a two-letter unmerged code (e.g.
	// "UU", "AU", "DD") for a conflicted entry.
	Status string `json:"status"`
}

// DiffFile is one file's diff as git computed it: both sides' content and
// git's hunks between them. The frontend only draws it.
type DiffFile struct {
	OldPath    string     `json:"oldPath,omitempty"` // rename/copy source; "" if not renamed
	Path       string     `json:"path"`
	Binary     bool       `json:"binary"`               // no content; Force never bypasses this
	TooLarge   bool       `json:"tooLarge"`             // no content unless Force
	OldContent string     `json:"oldContent,omitempty"` // "" if absent on this side (add/delete/root commit), or Binary/TooLarge
	NewContent string     `json:"newContent,omitempty"`
	Hunks      []DiffHunk `json:"hunks,omitempty"` // git's changes from OldContent to NewContent, in order; none for Binary, TooLarge or equal sides
	// Fingerprint identifies OldContent and NewContent together; a
	// LineSelection sends it back. "" without content.
	Fingerprint string `json:"fingerprint,omitempty"`
}

// DiffHunk is one git diff -U0 hunk header, git's numbers as printed. A side
// with no lines has Start = the line after which the other side's lines go
// (0: before the first line).
type DiffHunk struct {
	OldStart int `json:"oldStart"`
	OldLines int `json:"oldLines"`
	NewStart int `json:"newStart"`
	NewLines int `json:"newLines"`
}

// DiffOptions tunes a Diff* call.
type DiffOptions struct {
	Force            bool `json:"force"`            // bypass the size cap
	IgnoreWhitespace bool `json:"ignoreWhitespace"` // hunks from git diff -w
}
