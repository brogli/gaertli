// Package watch detects outside changes to a repo's git dir and a
// worktree's status: a repo-level RepoWatcher (fs events plus a poll
// fallback) and a worktree-level WorktreePoller (a focused window's status
// poll).
package watch

import "time"

// Timing knobs. Vars so tests can shorten them before constructing the
// watcher or poller under test.
var (
	DebounceInterval     = 300 * time.Millisecond
	FallbackPollInterval = 5 * time.Second // insurance only, not the primary path
	PollInterval         = 1 * time.Second
	MaxPollInterval      = 10 * time.Second // backoff cap
)
