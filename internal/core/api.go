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

	// Refs lists every local branch, every remote's tracking branches, and
	// every tag (FR-LOG-01's sidebar half; spec 10 delivers the per-commit
	// row-label half). Not streamed, not cached in session.Repo — unlike
	// LoadLog/LogRows: one for-each-ref call costs the same single process
	// spawn no matter how many branches exist, so recomputing on every
	// call is as cheap as a cache would be.
	Refs(ctx context.Context, wt WorktreeID) (Refs, error)

	// CommandLog returns the buffered entries so far (FR-UX-02): a window
	// that opens the panel after some commands already ran does not miss
	// them; new ones after that arrive via the "command" event.
	CommandLog(ctx context.Context, wt WorktreeID) ([]CommandLogEntry, error)

	// LoadLog starts loading wt's repo's commit log and graph if not
	// already loading or loaded (idempotent per repo, ADR-9's shared Repo
	// session). Returns once the load has started, not once it finishes;
	// rows arrive incrementally, readable via LogRows and announced by
	// "log:progress" events.
	LoadLog(ctx context.Context, wt WorktreeID) error

	// LogRows returns rows [from, to) of what has loaded so far
	// (FR-LOG-09): the window is clamped to [0, loaded), never an error for
	// a window past the current end. Error is set (Done also true) if the
	// load itself failed (FR-UX-01).
	LogRows(ctx context.Context, wt WorktreeID, from, to int) (LogRowsResult, error)

	// CommitDetails returns one commit's full message, committer, parents,
	// and changed files vs. its first parent (FR-LOG-03).
	CommitDetails(ctx context.Context, wt WorktreeID, sha string) (CommitDetails, error)

	// Refresh forces an immediate check of wt's repo and worktree for
	// outside changes (FR-REPO-04: manual refresh). Does not wait for any
	// resulting "repo:changed" event — those follow the normal path if
	// something actually differs.
	Refresh(ctx context.Context, wt WorktreeID) error

	// SetFocused tells wt's session whether its window currently has focus
	// (FR-REPO-04's "immediately on focus"; QA-PERF-06's CPU bound):
	// focused=true runs an immediate poller check and resumes its timer;
	// focused=false stops the timer.
	SetFocused(ctx context.Context, wt WorktreeID, focused bool) error

	// CloseWorktree ends wt's session (ADR-9): stops its poller, and — if
	// no other open worktree shares its repo — the repo's watcher too. A
	// no-op for an unknown id.
	CloseWorktree(ctx context.Context, wt WorktreeID) error

	// ReloadLog re-checks wt's repo's ref tips (cheap: for-each-ref, not a
	// commit walk) against the snapshot taken when the log last finished,
	// and restarts the log/graph stream only if they differ. No-op if
	// nothing has finished loading yet (call LoadLog first) or a load is
	// already running.
	ReloadLog(ctx context.Context, wt WorktreeID) error

	// LogIndexOf finds sha's row index in wt's repo's currently loaded log
	// (FR-LOG-01: clicking a ref in the sidebar selects and scrolls to its
	// commit). A linear scan over already-streamed rows, not a git call.
	// Found is false if sha is not, or not yet, among the loaded rows.
	LogIndexOf(ctx context.Context, wt WorktreeID, sha string) (LogIndexResult, error)
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
