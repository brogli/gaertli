package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/editor"
)

// SetEditorProfile implements core.API: installs the profile this core uses
// for every OpenFolder/OpenFile call it serves (TR-13).
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

// OpenFolder implements core.API (FR-ED-01, FR-ED-03).
func (c *Core) OpenFolder(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return editor.OpenFolder(c.currentProfile(), wt.Path)
}

// OpenFolderAt implements core.API (FR-WT-04).
func (c *Core) OpenFolderAt(ctx context.Context, path core.RepoPath) error {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return err
	}
	return editor.OpenFolder(c.currentProfile(), toplevel)
}

// OpenFile implements core.API (FR-ED-02, FR-ED-04). path is a
// ChangedFile.Path, relative to the worktree. FileNotFound covers a file
// deleted or moved away since the diff being viewed; a rename's Path is
// already the current name, so only Path is ever opened, never OldPath.
func (c *Core) OpenFile(ctx context.Context, id core.WorktreeID, path string, line int) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	full, err := worktreeFile(wt.Path, path)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(full); statErr != nil {
		return core.Errorf(core.FileNotFound, "%s no longer exists in this worktree", path)
	}
	return editor.OpenFile(c.currentProfile(), full, line)
}

// worktreeFile joins path onto dir and rejects one that resolves outside
// it — defense in depth (path is caller-supplied UI state, not a
// git-validated argument the way a revision or pathspec is, TR-10's spirit
// applied to a filesystem path instead).
func worktreeFile(dir, path string) (string, error) {
	full := filepath.Join(dir, path)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", core.Errorf(core.Internal, "path escapes the worktree: %s", path)
	}
	return full, nil
}
