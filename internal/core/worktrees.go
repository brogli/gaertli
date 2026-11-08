package core

import "context"

// Worktrees is the window's worktree (summary, refresh) and the repo's
// worktrees (list, add, remove, prune).
type Worktrees interface {
	// Summary is the worktree's branch and state, without ahead/behind and
	// refs. Cheap: the UI calls it again on every change.
	Summary(ctx context.Context, wt WorktreeID) (Summary, error)

	// Refresh checks wt's repo and worktree for outside changes now. Any
	// resulting "repo:changed" event follows asynchronously.
	Refresh(ctx context.Context, wt WorktreeID) error

	// ListWorktrees lists every worktree of path's repo: branch or detached
	// commit, clean/dirty, main/locked/missing. Stateless: starts no
	// session or poller.
	ListWorktrees(ctx context.Context, path RepoPath) ([]Worktree, error)

	// AddWorktree creates a worktree at target on the existing branch (by
	// name: a full ref would detach), or, if newBranch, on a new branch
	// started at start (as for CreateBranch). target must not exist
	// or must be empty (git's refusal). A new branch name is validated
	// first (Code InvalidName): worktree add -b's argument is not protected
	// by --.
	AddWorktree(ctx context.Context, path RepoPath, target, branch, start string, newBranch bool) (Worktree, error)

	// RemoveWorktree removes the worktree at target in path's repo. Without
	// force, a dirty working tree is Code DirtyWorktree; force discards it.
	// A locked worktree refuses either way.
	RemoveWorktree(ctx context.Context, path RepoPath, target string, force bool) error

	// PruneWorktrees removes bookkeeping for path's repo's worktrees whose
	// directory is gone.
	PruneWorktrees(ctx context.Context, path RepoPath) error
}

// Sessions opens, focuses and closes worktree sessions; only the shell calls
// it.
type Sessions interface {
	// OpenWorktree validates path and returns the session's identity.
	// Idempotent: opening an already-open worktree returns the same ID.
	OpenWorktree(ctx context.Context, path RepoPath) (WorktreeInfo, error)

	// SetFocused starts (with an immediate check) or stops wt's status
	// poller as its window gains or loses focus.
	SetFocused(ctx context.Context, wt WorktreeID, focused bool) error

	// CloseWorktree ends wt's session: its poller, and the repo's watcher if
	// no other open worktree shares the repo. No-op for an unknown id.
	CloseWorktree(ctx context.Context, wt WorktreeID) error

	// MainWorktree finds a repo's main worktree from its common git dir,
	// for when the repo's remembered worktree is gone. A bare repo with no
	// worktree left is Code NotARepo.
	MainWorktree(ctx context.Context, gitDir RepoPath) (Worktree, error)
}

// Worktree is one row of ListWorktrees/AddWorktree's result.
type Worktree struct {
	Path    string `json:"path"`
	Branch  string `json:"branch,omitempty"` // "" when detached
	SHA     SHA    `json:"sha"`
	Main    bool   `json:"main"` // git lists the main worktree first
	Locked  bool   `json:"locked"`
	Missing bool   `json:"missing"` // git's "prunable"; Dirty is always false then, the directory is gone
	Dirty   bool   `json:"dirty"`
	Bare    bool   `json:"bare,omitempty"` // git's "bare": a bare repo's own entry; no files, so Dirty is always false
}

// WorktreeInfo identifies an opened worktree session.
type WorktreeInfo struct {
	ID      WorktreeID `json:"id"`
	Path    string     `json:"path"`
	RepoKey string     `json:"repoKey"` // common git dir; shell bookkeeping (Registry.SetRepo), opaque to the UI
}

// Summary is the worktree's branch and state.
type Summary struct {
	Branch       string    `json:"branch"` // "" when Detached
	Detached     bool      `json:"detached"`
	Operation    Operation `json:"operation"`     // "" if none
	SHA          SHA       `json:"sha,omitempty"` // HEAD's commit
	EditorName   string    `json:"editorName"`    // the effective editor profile's Name
	MessageDraft string    `json:"messageDraft"`  // "" if none

	// WaitingStash is the commit of the newest stash entry a pull, merge or rebase with stash made
	// that is still in the stash list, "" if none.
	WaitingStash SHA `json:"waitingStash"`
}
