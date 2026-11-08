package core

import "context"

// Branches is the refs (branches, remote branches, tags) and what moves HEAD
// or a branch: create, check out, rename, delete, reset.
type Branches interface {
	// Refs lists local branches, remote-tracking branches, and tags. Not
	// cached: three git calls, whatever the ref count.
	Refs(ctx context.Context, wt WorktreeID) (Refs, error)

	// CreateBranch creates name at start (any revision git resolves; a
	// branch or tag as its full ref name, since git refuses a short name
	// that is both) and, if checkout, switches to it in the same atomic
	// call. An invalid name is Code InvalidName, checked before any write;
	// a duplicate is GitFailed; local changes blocking the switch are
	// DirtyWorktree.
	CreateBranch(ctx context.Context, wt WorktreeID, name, start string, checkout bool) error

	// Checkout switches wt to branch: a local branch's name, or a
	// remote-tracking branch's full ref name (refs/remotes/<remote>/<name>),
	// which becomes a new local branch tracking it. Local changes in the
	// way are DirtyWorktree; anything else (unknown name, a local branch of
	// that name already there, branch checked out in another worktree) is
	// GitFailed. No pre-check, no stash.
	Checkout(ctx context.Context, wt WorktreeID, branch string) error

	// CheckoutCommit detaches wt's HEAD at sha. Same errors as Checkout.
	CheckoutCommit(ctx context.Context, wt WorktreeID, sha SHA) error

	// Reset moves wt's current branch, or its detached HEAD, to sha.
	// Refusals are GitFailed, e.g. --soft during a merge.
	// --mixed and --hard never refuse a dirty tree or a conflict.
	Reset(ctx context.Context, wt WorktreeID, sha SHA, mode ResetMode) (ResetResult, error)

	// RenameBranch renames name to newName, validated first (Code
	// InvalidName).
	RenameBranch(ctx context.Context, wt WorktreeID, name, newName string) error

	// DeleteBranch deletes name. Without force, a branch with commits not
	// merged into its upstream, or into HEAD if it has none, is Code
	// Unmerged. A branch checked out in any worktree refuses either way
	// (GitFailed).
	DeleteBranch(ctx context.Context, wt WorktreeID, name string, force bool) error
}

// Refs is the repo's branches, remotes, and tags.
type Refs struct {
	Branches []Branch `json:"branches"`
	Remotes  []Remote `json:"remotes"`
	Tags     []Tag    `json:"tags"`
}

// Branch is one local branch.
type Branch struct {
	Name         string `json:"name"`
	SHA          SHA    `json:"sha"`
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
	SHA  SHA    `json:"sha"`
}

// Tag is one tag. SHA is always the commit it points to, also for an
// annotated tag.
type Tag struct {
	Name string `json:"name"`
	SHA  SHA    `json:"sha"`
}

// ResetResult is Reset's result: the HEAD commit before the reset, so the
// user can go back. "" only if HEAD was unborn.
type ResetResult struct {
	PreviousHead SHA `json:"previousHead"`
}

// ResetMode is git reset's mode.
type ResetMode string

const (
	ResetSoft  ResetMode = "soft"
	ResetMixed ResetMode = "mixed"
	ResetHard  ResetMode = "hard"
	ResetKeep  ResetMode = "keep"
)
