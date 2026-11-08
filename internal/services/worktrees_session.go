package services

import (
	"context"

	"github.com/brogli/gaertli/internal/core"
)

// OpenWorktree implements core.API.
func (c *Core) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	wt, err := c.sessions.Open(ctx, path)
	if err != nil {
		return core.WorktreeInfo{}, err
	}
	return core.WorktreeInfo{ID: wt.ID, Path: wt.Path, RepoKey: wt.Repo.Key}, nil
}

// Summary implements core.API.
func (c *Core) Summary(ctx context.Context, id core.WorktreeID) (core.Summary, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.Summary{}, err
	}

	branch, detached, err := headBranch(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Summary{}, err
	}
	sha, err := verifyRev(ctx, wt.Runner, wt.Path, "HEAD")
	if err != nil {
		return core.Summary{}, err
	}
	stash, err := waitingStash(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Summary{}, err
	}
	return core.Summary{
		Branch: branch, Detached: detached, Operation: core.OperationKind(operation(wt.GitDir)), SHA: sha,
		EditorName: c.currentProfile().Name, MessageDraft: wt.MessageDraft(), WaitingStash: stash,
	}, nil
}

// Refresh implements core.API.
func (c *Core) Refresh(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Repo.Watcher().CheckNow()
	wt.Poller().CheckNow()
	return nil
}

// SetFocused implements core.API.
func (c *Core) SetFocused(ctx context.Context, id core.WorktreeID, focused bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if focused {
		wt.Poller().Focus()
	} else {
		wt.Poller().Blur()
	}
	return nil
}

// CloseWorktree implements core.API.
func (c *Core) CloseWorktree(ctx context.Context, id core.WorktreeID) error {
	c.sessions.Close(id)
	return nil
}
