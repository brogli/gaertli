package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/editor"
)

// SetEditorProfile implements core.API: installs the profile this core uses
// for every OpenFolder/OpenFile call it serves (TR-13, pushed by the shell
// on connect).
func (c *Core) SetEditorProfile(ctx context.Context, profile core.EditorProfile) error {
	c.editorProfile.Store(profile)
	return nil
}

// currentProfile is the zero-value safety net: in normal operation the
// shell always pushes a profile before this matters (Run, at startup;
// DistroCore.OnConnect, at every connect) — this only covers a core used
// before its first push, which should not happen in practice.
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

// OpenFile implements core.API (FR-ED-04).
func (c *Core) OpenFile(ctx context.Context, id core.WorktreeID, path string, line int) error {
	if _, err := c.sessions.Get(id); err != nil { // still requires a valid worktree, even though path is caller-supplied
		return err
	}
	return editor.OpenFile(c.currentProfile(), path, line)
}
