package core

// EditorProfile is one environment's editor launch configuration, reused
// verbatim by internal/settings and internal/editor.
type EditorProfile struct {
	Name          string `json:"name"`
	FolderCmd     string `json:"folderCmd"`
	FileCmd       string `json:"fileCmd"`
	FileAtLineCmd string `json:"fileAtLineCmd,omitempty"` // falls back to FileCmd if empty
}

// EditorRecord is one editor command a core started, streamed to the shell
// via the "editor" event for the app log.
type EditorRecord struct {
	Argv  []string `json:"argv"`
	Error string   `json:"error,omitempty"` // "" on success
}
