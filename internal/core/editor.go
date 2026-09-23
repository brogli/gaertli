package core

// EditorProfile is one environment's editor launch configuration, reused
// verbatim by internal/settings and internal/editor.
type EditorProfile struct {
	Name          string `json:"name"`
	FolderCmd     string `json:"folderCmd"`
	FileCmd       string `json:"fileCmd"`
	FileAtLineCmd string `json:"fileAtLineCmd,omitempty"` // falls back to FileCmd if empty
}

// EditorFallbackKind selects OpenFolderFallback/OpenFileFallback's
// substitute editor, offered after OpenFolder/OpenFile itself returned
// EditorNotFound.
type EditorFallbackKind string

const (
	FallbackGitEditor EditorFallbackKind = "git"    // git var GIT_EDITOR
	FallbackSystem    EditorFallbackKind = "system" // OS default opener
)
