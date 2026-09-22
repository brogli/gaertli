// Package watch detects outside changes to a repo's git dir and a
// worktree's status: a repo-level RepoWatcher (fs events plus a poll
// fallback) and a worktree-level WorktreePoller (a focused window's status
// poll).
package watch

import "time"

// Scope is what part of the repo's git state changed.
type Scope string

const (
	Refs      Scope = "refs"      // HEAD, refs/, packed-refs — any worktree's
	Index     Scope = "index"     // a worktree's index file, or its working tree (status poll)
	Worktrees Scope = "worktrees" // an entry added/removed under worktrees/
	State     Scope = "state"     // MERGE_HEAD, CHERRY_PICK_HEAD, REVERT_HEAD, rebase-*, sequencer/
)

// Timing knobs. Vars so tests can shorten them before constructing the
// watcher or poller under test.
var (
	DebounceInterval     = 300 * time.Millisecond
	FallbackPollInterval = 5 * time.Second // insurance only, not the primary path
	PollInterval         = 1 * time.Second
	MaxPollInterval      = 10 * time.Second // backoff cap
)
