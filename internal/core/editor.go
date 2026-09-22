package core

// EditorProfile is one environment's editor launch configuration
// (git-mapping.md), reused verbatim by internal/settings and
// internal/editor.
type EditorProfile struct {
	Name          string `json:"name"`
	FolderCmd     string `json:"folderCmd"`
	FileCmd       string `json:"fileCmd"`
	FileAtLineCmd string `json:"fileAtLineCmd,omitempty"` // falls back to FileCmd if empty
}
