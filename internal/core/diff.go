package core

// DiffFile is one file's two-sided content. The frontend computes the
// visual diff from OldContent/NewContent; Go only fetches, classifies, and
// caps.
type DiffFile struct {
	OldPath    string `json:"oldPath,omitempty"` // rename/copy source; "" if not renamed
	Path       string `json:"path"`
	Binary     bool   `json:"binary"`               // no content; force never bypasses this
	TooLarge   bool   `json:"tooLarge"`             // no content unless force
	OldContent string `json:"oldContent,omitempty"` // "" if absent on this side (add/delete/root commit), or Binary/TooLarge
	NewContent string `json:"newContent,omitempty"`
}
