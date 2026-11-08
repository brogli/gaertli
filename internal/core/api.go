// Package core is the backend contract: one interface per bounded context,
// JSON-serialisable DTOs, one error type, and events.
//
// Each context's file holds its interface and its types: Log (log.go),
// Changes (changes.go), Branches (branches.go), Sync (sync.go), Operations
// (operations.go), Conflicts (conflicts.go), Worktrees (worktrees.go) and
// Editor (editor.go). diff.go holds the types Log and Changes share.
//
// Implemented in-process by internal/services and over JSON-RPC by
// internal/remote.Client. The shell binds the generated app.Forwarder, which
// forwards each call to the core that owns the call's worktree or path.
package core

import "context"

//go:generate go run ../../tools/gencore

// WindowAPI is what a window may call; the shell binds it to Wails.
type WindowAPI interface {
	Log
	Changes
	Branches
	Sync
	Operations
	Conflicts
	Worktrees
	Editor

	// Version identifies the core; the shell compares Protocol on connect.
	Version(ctx context.Context) (Version, error)
}

// API is the whole backend: WindowAPI and the calls only the shell makes.
// Rules, enforced by tools/gencore: first parameter context.Context, last
// result error, at most one other result, and every parameter and result
// JSON-serialisable (no chan, func, or interface).
//
// The second parameter decides where the facade sends the call: a WorktreeID
// goes to the core owning that worktree, a RepoPath to the core for that path,
// anything else to the local core.
type API interface {
	WindowAPI
	Sessions
	EditorSetup
}

// Version of a core. App and Commit are "dev" and "" until a release build
// stamps them. Git is the version of the git the core runs, "" when it is
// missing or did not report readably; GitOutdated says a git that runs is
// from an older release cycle than the latest.
type Version struct {
	App         string `json:"app"`
	Commit      string `json:"commit"`
	Protocol    int    `json:"protocol"`
	Git         string `json:"git"`
	GitOutdated bool   `json:"gitOutdated"`
}

// Protocol is bumped whenever the wire format of API changes.
const Protocol = 6

// WorktreeID names a worktree session. Issued by the core that owns the
// repo; opaque to the UI. The shell and the UI name a WSL distro's
// worktree by its core's id in UNC form (\\wsl.localhost\<distro>\...),
// which routes each call to that distro's core; the shell hands the core
// back its own id.
type WorktreeID string

// RepoPath is a user-supplied path, classified by the shell as local or WSL.
type RepoPath string
