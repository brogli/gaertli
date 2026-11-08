package services

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/editor"
)

// SetEditorProfile implements core.API.
func (c *Core) SetEditorProfile(ctx context.Context, profile core.EditorProfile) error {
	c.editorProfile.Store(profile)
	return nil
}

// currentProfile is the profile the shell last pushed; empty before the
// first push, so every open-in-editor action then reports that no editor is
// configured.
func (c *Core) currentProfile() core.EditorProfile {
	p, _ := c.editorProfile.Load().(core.EditorProfile)
	return p
}

// OpenFolder implements core.API.
func (c *Core) OpenFolder(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	argv, err := editor.OpenFolder(c.currentProfile(), wt.Path)
	c.logEditorStart(id, argv, err)
	return err
}

// OpenFolderAt implements core.API.
func (c *Core) OpenFolderAt(ctx context.Context, path core.RepoPath) error {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return err
	}
	argv, err := editor.OpenFolder(c.currentProfile(), toplevel)
	c.logEditorStart(core.WorktreeID(toplevel), argv, err)
	return err
}

// OpenFile implements core.API. path is a ChangedFile.Path: for a rename,
// already the current name.
func (c *Core) OpenFile(ctx context.Context, id core.WorktreeID, path string, line int) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	full, err := worktreeFile(wt.Path, path)
	if err != nil {
		return err
	}
	argv, err := editor.OpenFile(c.currentProfile(), full, line)
	c.logEditorStart(id, argv, err)
	return err
}

// logEditorStart reports one editor start, or its failure, to the shell's
// app log via the "editor" event, the same way a git command's own log
// line travels. argv nil means the command was never built (no start to
// report).
func (c *Core) logEditorStart(id core.WorktreeID, argv []string, err error) {
	if argv == nil {
		return
	}
	record := core.EditorRecord{Argv: argv}
	if err != nil {
		record.Error = err.Error()
	}
	payload, _ := json.Marshal(record)
	c.sink(core.Event{Name: "editor", Worktree: id, Payload: payload})
}

// worktreeFile joins path onto dir. A path that resolves outside dir is
// Code Internal: path is caller-supplied and, unlike a revision or
// pathspec, never validated by git. A file deleted or moved away since the
// diff being viewed is Code FileNotFound.
func worktreeFile(dir, path string) (string, error) {
	full := filepath.Join(dir, path)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", core.Errorf(core.Internal, "path escapes the worktree: %s", path)
	}
	if !fileExists(full) {
		return "", core.ErrorfArgs(core.FileNotFound, "fileNotFound", map[string]string{"path": path}, "%s no longer exists in this worktree", path)
	}
	return full, nil
}
