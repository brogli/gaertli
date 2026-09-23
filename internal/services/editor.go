package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/editor"
	"github.com/brogli/nebegit/internal/git"
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

// TestEditorProfile implements core.API.
func (c *Core) TestEditorProfile(ctx context.Context, profile core.EditorProfile) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return core.Errorf(core.Internal, "could not determine the home directory: %v", err)
	}
	return editor.OpenFolder(profile, home)
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
	if _, err := os.Stat(full); err != nil {
		return "", core.Errorf(core.FileNotFound, "%s no longer exists in this worktree", path)
	}
	return full, nil
}

// gitEditorProfile wraps dir's git editor (`git var GIT_EDITOR`: GIT_EDITOR,
// VISUAL, EDITOR, core.editor, or git's default) in a profile that appends
// {path}, the way git invokes it.
func gitEditorProfile(ctx context.Context, r git.Runner, dir string) (core.EditorProfile, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: []string{"var", "GIT_EDITOR"}})
	if err != nil {
		return core.EditorProfile{}, err
	}
	cmd := strings.TrimSpace(string(result.Stdout)) + " {path}"
	return core.EditorProfile{Name: "git editor", FolderCmd: cmd, FileCmd: cmd}, nil
}

// fallbackProfile resolves kind to a profile: the OS default opener, or
// dir's git-configured editor.
func fallbackProfile(ctx context.Context, r git.Runner, dir string, kind core.EditorFallbackKind) (core.EditorProfile, error) {
	if kind == core.FallbackSystem {
		return editor.SystemProfile(), nil
	}
	return gitEditorProfile(ctx, r, dir)
}

// OpenFolderFallback implements core.API.
func (c *Core) OpenFolderFallback(ctx context.Context, id core.WorktreeID, kind core.EditorFallbackKind) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	profile, err := fallbackProfile(ctx, wt.Runner, wt.Path, kind)
	if err != nil {
		return err
	}
	return editor.OpenFolder(profile, wt.Path)
}

// OpenFileFallback implements core.API.
func (c *Core) OpenFileFallback(ctx context.Context, id core.WorktreeID, path string, line int, kind core.EditorFallbackKind) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	full, err := existingWorktreeFile(wt.Path, path)
	if err != nil {
		return err
	}
	profile, err := fallbackProfile(ctx, wt.Runner, wt.Path, kind)
	if err != nil {
		return err
	}
	return editor.OpenFile(profile, full, 0)
}
