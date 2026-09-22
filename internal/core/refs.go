package core

// Refs is the repo's branches, remotes, and tags for the sidebar (FR-LOG-01).
type Refs struct {
	Branches []Branch `json:"branches"`
	Remotes  []Remote `json:"remotes"`
	Tags     []Tag    `json:"tags"`
}

// Branch is one local branch.
type Branch struct {
	Name         string `json:"name"`
	SHA          string `json:"sha"`
	Upstream     string `json:"upstream,omitempty"` // short form; "" if none configured
	Ahead        int    `json:"ahead"`
	Behind       int    `json:"behind"`
	UpstreamGone bool   `json:"upstreamGone"` // upstream configured but its ref no longer exists
	Current      bool   `json:"current"`      // this worktree's own HEAD
	// CheckedOutIn is the one worktree with this branch checked out, ""
	// if none — a single string, not a slice: a branch is checked out in
	// at most one worktree at a time, unlike a detached commit SHA
	// (LogCommit.CheckedOutIn).
	CheckedOutIn string `json:"checkedOutIn,omitempty"`
}

// Remote is one remote's tracking branches, grouped (FR-LOG-01).
type Remote struct {
	Name     string         `json:"name"`
	Branches []RemoteBranch `json:"branches"`
}

// RemoteBranch is one remote-tracking branch; Name excludes the remote
// prefix (e.g. "main" under Remote{Name: "origin"}, from "origin/main").
type RemoteBranch struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// Tag is one tag. SHA is always the commit it points to: for an
// annotated tag the tag object's own SHA is dereferenced — otherwise
// "jump to this commit" would search the log for a SHA no commit row has.
type Tag struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// LogIndexResult is LogIndexOf's result.
type LogIndexResult struct {
	Index int  `json:"index"`
	Found bool `json:"found"`
}
