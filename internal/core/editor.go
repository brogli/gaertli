package core

import "context"

// Editor starts the external editor.
type Editor interface {
	// OpenFolder starts the configured editor on wt's folder.
	OpenFolder(ctx context.Context, wt WorktreeID) error

	// OpenFile starts the configured editor on path (worktree-relative), at
	// line if line > 0.
	OpenFile(ctx context.Context, wt WorktreeID, path string, line int) error

	// OpenFolderAt starts the configured editor on path, a worktree with no
	// session (a worktree picker row other than the window's own).
	OpenFolderAt(ctx context.Context, path RepoPath) error
}

// EditorSetup sets the editor profile; only the shell calls it.
type EditorSetup interface {
	// SetEditorProfile sets the profile this core's OpenFolder/OpenFile use.
	// The shell calls it directly on each core when connecting: the
	// generated Forwarder would route it to the local core only.
	SetEditorProfile(ctx context.Context, profile EditorProfile) error
}

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
