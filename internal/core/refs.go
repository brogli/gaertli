package core

// Refs is the repo's branches, remotes, and tags.
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
	// CheckedOutIn is the worktree holding this branch, "" if none: git
	// checks a branch out in at most one worktree. Also set, with InUse,
	// for a worktree that has the branch's HEAD detached mid-rebase or
	// mid-bisect.
	CheckedOutIn string      `json:"checkedOutIn,omitempty"`
	InUse        BranchInUse `json:"inUse,omitempty"` // "" unless CheckedOutIn is mid-rebase or mid-bisect
}

// BranchInUse is why CheckedOutIn's worktree has the branch's HEAD detached
// instead of the branch itself checked out.
type BranchInUse string

const (
	BranchRebasing  BranchInUse = "rebase"
	BranchBisecting BranchInUse = "bisect"
)

// Remote is one configured remote (git remote) with its tracking branches,
// grouped; Branches is empty for a remote with none (e.g. just added).
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

// Tag is one tag. SHA is always the commit it points to, also for an
// annotated tag.
type Tag struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// LogIndexResult is LogIndexOf's result.
type LogIndexResult struct {
	Index int  `json:"index"`
	Found bool `json:"found"`
}
