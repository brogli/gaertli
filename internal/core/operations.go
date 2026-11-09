package core

import "context"

// Operations change the current branch by merging or replaying commits: the
// operation a stop leaves in progress, and the waiting stash.
type Operations interface {
	// Merge merges ref (a full ref name: refs/heads/x, refs/remotes/o/x,
	// refs/tags/x) into wt's current branch: a fast-forward when possible,
	// else a merge commit with git's default message; hooks run. Refused
	// before git runs: an operation in progress (GitFailed, key
	// operationInProgress), a detached HEAD (GitFailed, key detachedHead).
	// Local changes in the way are DirtyWorktree (key mergeBlockedByChanges)
	// unless stash: then they are stashed first and restored after a clean
	// merge, as PullWithStash does. A stop is left for the conflict flow:
	// Code Conflicted while files conflict, else Code Stopped with git's
	// reason.
	Merge(ctx context.Context, wt WorktreeID, ref string, stash bool) (IntegrateResult, error)

	// Rebase rebases wt's current branch onto ref, flattening merge commits
	// and moving no other branch. Same refusals, stash and stops as Merge
	// (DirtyWorktree key rebaseBlockedByChanges).
	Rebase(ctx context.Context, wt WorktreeID, ref string, stash bool) (IntegrateResult, error)

	// CherryPick applies shas onto wt's HEAD, oldest first by committer date
	// (same-second ties in topological order). A merge commit
	// among shas refuses the whole call before git runs. A stop that leaves
	// CHERRY_PICK_HEAD (a conflict or an empty result) is Code Conflicted,
	// left in place; any other refusal is GitFailed.
	CherryPick(ctx context.Context, wt WorktreeID, shas []string, opts PickOptions) error

	// Revert is CherryPick's counterpart, applied newest first, with the
	// same merge refusal and errors; opts.RecordOrigin is ignored.
	Revert(ctx context.Context, wt WorktreeID, shas []string, opts PickOptions) error

	// SquashCheck tells whether Squash and Fixup can fold shas (two or more
	// commits, any order), and what to say first. Blocker, in this order: an
	// operation in progress, a detached HEAD, a sha not reachable from HEAD
	// or shas with no common ancestor, a merge commit between the oldest
	// sha and HEAD, uncommitted changes to tracked files (DirtyWorktree).
	// Every other blocker is GitFailed. Reads only.
	SquashCheck(ctx context.Context, wt WorktreeID, shas []string) (SquashCheck, error)

	// Squash folds shas into one commit with message, exactly as given
	// apart from surrounding whitespace (a prepare-commit-msg hook can
	// still change it, as for any commit), in the oldest sha's place and with
	// its author; the other commits between it and HEAD are replayed in
	// their order after it. It runs git rebase -i with a prepared todo
	// under the write lock; no editor ever starts. A blocker (SquashCheck)
	// refuses before any write. A rebase that stops is left for the resolve
	// flow: Code Conflicted when files conflict, else Code Stopped with git's
	// reason. A blank message, or fewer than two distinct shas, is Code
	// Internal.
	Squash(ctx context.Context, wt WorktreeID, shas []string, message string) (SquashResult, error)

	// Fixup is Squash keeping the oldest sha's message exactly.
	Fixup(ctx context.Context, wt WorktreeID, shas []string) (SquashResult, error)

	// ContinueOperation runs --continue for wt's in-progress merge, rebase,
	// cherry-pick, or revert (Summary.Operation) under the write lock, with
	// GIT_EDITOR=true so no editor waits. No operation in progress is Code
	// GitFailed (key noOperation). A step that leaves the operation in
	// progress is a stop: Code Conflicted while files conflict, else Code
	// Stopped with git's reason.
	ContinueOperation(ctx context.Context, wt WorktreeID) error

	// AbortOperation runs --abort the same way, back to the pre-operation
	// HEAD. The UI confirms first.
	AbortOperation(ctx context.Context, wt WorktreeID) error

	// SkipOperation skips the current commit of a stopped cherry-pick or
	// revert. Any other operation is Code GitFailed (key
	// operationNotSkippable): merge has no --skip, and rebase is not
	// supported.
	SkipOperation(ctx context.Context, wt WorktreeID) error

	// PopStash applies the stash entry whose commit is sha and drops it on
	// success; sha is resolved to its current stash@{n} at call time. A
	// conflicting pop keeps the entry. Code Internal if no entry has sha.
	PopStash(ctx context.Context, wt WorktreeID, sha string) error

	// DropStash deletes the stash entry sha, resolved like PopStash. The UI
	// confirms first.
	DropStash(ctx context.Context, wt WorktreeID, sha string) error
}

// Operation is a merge, rebase, cherry-pick or revert in progress.
type Operation string

const (
	MergeOperation      Operation = "merge"
	RebaseOperation     Operation = "rebase"
	CherryPickOperation Operation = "cherry-pick"
	RevertOperation     Operation = "revert"
)

// IntegrateResult is Merge's and Rebase's result.
type IntegrateResult struct {
	Outcome IntegrateOutcome `json:"outcome"`
}

// IntegrateOutcome is what Merge or Rebase did to the current branch.
type IntegrateOutcome string

const (
	IntegrateUpToDate    IntegrateOutcome = "upToDate"    // HEAD did not move
	IntegrateFastForward IntegrateOutcome = "fastForward" // Merge moved HEAD to the merged ref
	IntegrateMerged      IntegrateOutcome = "merged"      // Merge made a merge commit
	IntegrateRebased     IntegrateOutcome = "rebased"
)

// PickOptions are CherryPick's and Revert's options; the zero value is git's
// defaults. RecordOrigin applies to cherry-pick only.
type PickOptions struct {
	NoCommit     bool `json:"noCommit"`     // -n: apply without committing
	RecordOrigin bool `json:"recordOrigin"` // -x: append "(cherry picked from commit <sha>)"
}

// SquashCheck says whether Squash and Fixup can fold a selection of
// commits, and what the user is told before they run.
type SquashCheck struct {
	// Blocker is the error Squash and Fixup would refuse with; nil if they
	// can run.
	Blocker *Error `json:"blocker,omitempty"`
	// Pushed: a selected commit is already on the branch's upstream, so the
	// branch needs a force push afterwards.
	Pushed bool `json:"pushed"`
	// Message is the selected commits' messages, oldest first, separated by
	// a blank line: what Squash proposes for the folded commit.
	Message string `json:"message"`
}

// SquashResult is Squash's and Fixup's result: the branch's commit before,
// so the user can go back.
type SquashResult struct {
	PreviousHead string `json:"previousHead"`
}
