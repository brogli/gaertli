package services

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/editor"
)

// SetEditorProfile implements core.API.
func (c *Core) SetEditorProfile(ctx context.Context, profile core.EditorProfile) error {
	c.editorProfile.Store(profile)
	return nil
}

// currentProfile falls back to the default profile for a core used before
// the shell's first SetEditorProfile push.
func (c *Core) currentProfile() core.EditorProfile {
	p, _ := c.editorProfile.Load().(core.EditorProfile)
	return editor.OrDefault(p)
}

// OpenFolder implements core.API.
func (c *Core) OpenFolder(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return editor.OpenFolder(c.currentProfile(), wt.Path)
}

// OpenFolderAt implements core.API.
func (c *Core) OpenFolderAt(ctx context.Context, path core.RepoPath) error {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return err
	}
	return editor.OpenFolder(c.currentProfile(), toplevel)
}

// OpenFile implements core.API. path is a ChangedFile.Path: for a rename,
// already the current name.
func (c *Core) OpenFile(ctx context.Context, id core.WorktreeID, path string, line int) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	full, err := existingWorktreeFile(wt.Path, path)
	if err != nil {
		return err
	}
	return editor.OpenFile(c.currentProfile(), full, line)
}

// worktreeFile joins path onto dir and rejects one that resolves outside
// it: path is caller-supplied and, unlike a revision or pathspec, never
// validated by git.
func worktreeFile(dir, path string) (string, error) {
	full := filepath.Join(dir, path)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", core.Errorf(core.Internal, "path escapes the worktree: %s", path)
	}
	return full, nil
}

// existingWorktreeFile is worktreeFile, plus Code FileNotFound for a file
// deleted or moved away since the diff being viewed.
func existingWorktreeFile(dir, path string) (string, error) {
	full, err := worktreeFile(dir, path)
	if err != nil {
		return "", err
	}
	if !fileExists(full) {
		return "", core.ErrorfArgs(core.FileNotFound, "fileNotFound", map[string]string{"path": path}, "%s no longer exists in this worktree", path)
	}
	return full, nil
}
