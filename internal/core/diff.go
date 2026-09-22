package core

// DiffFile is one file's two-sided content at a commit (FR-LOG-04, ADR-7).
// CodeMirror computes the visual diff client-side from OldContent/NewContent
// (ADR-7); Go only fetches, classifies, and caps.
type DiffFile struct {
	OldPath    string `json:"oldPath,omitempty"` // rename/copy source; "" if not renamed
	Path       string `json:"path"`
	Binary     bool   `json:"binary"`               // FR-DIFF-03: no content, force never bypasses this
	TooLarge   bool   `json:"tooLarge"`             // FR-DIFF-04: no content unless force
	OldContent string `json:"oldContent,omitempty"` // "" if absent on this side (add/delete/root commit), or Binary/TooLarge
	NewContent string `json:"newContent,omitempty"`
}
