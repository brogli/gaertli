// Package core is the backend contract: one interface, JSON-serialisable
// DTOs, one error type, and events (TR-28, ADR-3).
//
// Implemented in-process by internal/services and over JSON-RPC by
// internal/remote.Client. The shell binds the generated app.Facade, which
// forwards each call to the core that owns the window's worktree.
package core

import "context"

//go:generate go run ../../tools/gencore

// API is the whole backend. Rules, enforced by tools/gencore: first parameter
// context.Context, last result error, at most one other result, and every
// parameter and result JSON-serialisable (no chan, func, or interface).
//
// The second parameter decides where the facade sends the call: a WorktreeID
// goes to the core owning that worktree, a RepoPath to the core for that path,
// anything else to the local core.
type API interface {
	// Version identifies the core; the shell compares Protocol on connect.
	Version(ctx context.Context) (Version, error)

	// OpenWorktree validates path and returns the session's identity
	// (FR-REPO-01, FR-REPO-05). Idempotent: opening an already-open
	// worktree returns the same ID.
	OpenWorktree(ctx context.Context, path RepoPath) (WorktreeInfo, error)

	// Summary is FR-REPO-03's "always show" panel minus ahead/behind (spec
	// 11) and the refs lists (10/11). Cheap and repeatable — spec 12's
	// watcher calls it again on change.
	Summary(ctx context.Context, wt WorktreeID) (Summary, error)

	// CommandLog returns the buffered entries so far (FR-UX-02): a window
	// that opens the panel after some commands already ran does not miss
	// them; new ones after that arrive via the "command" event.
	CommandLog(ctx context.Context, wt WorktreeID) ([]CommandLogEntry, error)
}

// Version of a core. App is "dev" until a release build stamps it.
type Version struct {
	App      string `json:"app"`
	Protocol int    `json:"protocol"`
}

// Protocol is bumped whenever the wire format of API changes.
const Protocol = 1

// WorktreeID names a worktree session (ADR-9). Issued by the core that owns
// the repo; opaque to the UI.
type WorktreeID string

// RepoPath is a user-supplied path, classified by the shell as local or WSL.
type RepoPath string

// WorktreeInfo identifies an opened worktree session.
type WorktreeInfo struct {
	ID      WorktreeID `json:"id"`
	Path    string     `json:"path"`
	RepoKey string     `json:"repoKey"` // common git dir; shell bookkeeping (Registry.SetRepo), opaque to the UI
}

// Summary is the worktree's always-shown branch/state panel (FR-REPO-03).
type Summary struct {
	Path      string `json:"path"`
	Branch    string `json:"branch"` // "" when Detached
	Detached  bool   `json:"detached"`
	Operation string `json:"operation"` // "", "merge", "rebase", "cherry-pick", "revert"
}
