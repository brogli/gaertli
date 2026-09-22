package core

// EditorProfile is one environment's editor launch configuration
// (git-mapping.md: {name, folderCmd, fileCmd, fileAtLineCmd?}). Reused
// verbatim by internal/settings and internal/editor (TR-18 DRY): the shape
// is identical in all three places.
type EditorProfile struct {
	Name          string `json:"name"`
	FolderCmd     string `json:"folderCmd"`
	FileCmd       string `json:"fileCmd"`
	FileAtLineCmd string `json:"fileAtLineCmd,omitempty"` // falls back to FileCmd if empty
}
