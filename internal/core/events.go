package core

import "encoding/json"

// Event goes from a core to the shell. Repo (the common git dir) targets all
// windows of a repo; Worktree, when set, targets one window (ADR-9).
type Event struct {
	Name     string          `json:"name"`
	Repo     string          `json:"repo,omitempty"`
	Worktree WorktreeID      `json:"worktree,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// EventSink receives every event a core emits. Every core takes one.
type EventSink func(Event)
