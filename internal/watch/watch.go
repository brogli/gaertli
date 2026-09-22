// Package watch detects outside changes to a repo's git dir and a
// worktree's status (FR-REPO-04, ADR-9): a repo-level RepoWatcher (fs
// events plus a poll fallback) and a worktree-level WorktreePoller (a
// focused window's status poll).
package watch

import "time"

// Scope is what part of the repo's git state changed (components.md).
type Scope string

const (
	Refs      Scope = "refs"      // HEAD, refs/, packed-refs — any worktree's
	Index     Scope = "index"     // a worktree's index file, or its working tree (status poll)
	Worktrees Scope = "worktrees" // an entry added/removed under worktrees/
	State     Scope = "state"     // MERGE_HEAD, CHERRY_PICK_HEAD, REVERT_HEAD, rebase-*, sequencer/
)

// Timing knobs (git-mapping.md). Declared as vars, not consts: the
// acceptance cases override them per-test for speed, restoring the default
// afterwards (TestMain/t.Cleanup would be process-wide; these are set
// before constructing the watcher/poller under test, which reads them once).
var (
	DebounceInterval     = 300 * time.Millisecond // git-mapping.md
	FallbackPollInterval = 5 * time.Second        // insurance only, not the primary path
	PollInterval         = 1 * time.Second        // git-mapping.md: "start at 1s"
	MaxPollInterval      = 10 * time.Second       // backoff cap
)
