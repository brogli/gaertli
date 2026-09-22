package core

import "encoding/json"

// Event goes from a core to the shell. Repo (the common git dir) targets all
// windows of a repo; Worktree, when set, targets the windows selecting it.
type Event struct {
	Name     string          `json:"name"`
	Repo     string          `json:"repo,omitempty"`
	Worktree WorktreeID      `json:"worktree,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// EventSink receives every event a core emits. Every core takes one.
type EventSink func(Event)

// RepoChanged is the payload of a "repo:changed" event.
type RepoChanged struct {
	Scope string `json:"scope"` // "refs" | "index" | "worktrees" | "state"
}
