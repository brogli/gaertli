// Package core is the backend contract: one interface, JSON-serialisable
// DTOs, one error type, and events.
//
// Implemented in-process by internal/services and over JSON-RPC by
// internal/remote.Client. The shell binds the generated app.Forwarder, which
// forwards each call to the core that owns the call's worktree or path.
package core

import "context"

//go:generate go run ../../tools/gencore

// WindowAPI is what a window may call; the shell binds it to Wails.
type WindowAPI interface {
	// Version identifies the core; the shell compares Protocol on connect.
	Version(ctx context.Context) (Version, error)

	// Summary is the worktree's branch and state, without ahead/behind and
	// refs. Cheap: the UI calls it again on every change.
	Summary(ctx context.Context, wt WorktreeID) (Summary, error)

	// Refs lists local branches, remote-tracking branches, and tags. Not
	// cached: two git calls, whatever the ref count.
	Refs(ctx context.Context, wt WorktreeID) (Refs, error)

	// LoadLog starts loading the repo's commit log and graph unless already
	// loading or loaded (idempotent per repo). Returns once the load has
	// started; rows are read via LogRows and announced by "log:progress".
	LoadLog(ctx context.Context, wt WorktreeID) error

	// LogRows returns rows [from, to) of what has loaded so far, clamped to
	// [0, loaded). Error (and Done) is set if the load failed.
	LogRows(ctx context.Context, wt WorktreeID, from, to int) (LogRowsResult, error)

	// CommitDetails returns one commit's full message, committer, parents,
	// and changed files vs. its first parent.
	CommitDetails(ctx context.Context, wt WorktreeID, sha string) (CommitDetails, error)

	// Status lists wt's uncommitted changes: staged, unstaged, untracked,
	// conflicted. A file changed both ways appears in Staged and Unstaged.
	Status(ctx context.Context, wt WorktreeID) (ChangedFiles, error)

	// StageFile (git add) and UnstageFile (git restore --staged) move one
	// path between the working tree and the index. Refusals are GitFailed.
	StageFile(ctx context.Context, wt WorktreeID, path string) error
	UnstageFile(ctx context.Context, wt WorktreeID, path string) error

	// StageAll (git add -A) and UnstageAll (git restore --staged -- .) act
	// on every changed file.
	StageAll(ctx context.Context, wt WorktreeID) error
	UnstageAll(ctx context.Context, wt WorktreeID) error

	// StageLines stages (or, if reverse, unstages) the changed lines sel
	// names in a fresh diff of path: the unstaged diff, or the staged diff
	// if reverse. If that diff's sides no longer match sel.Fingerprint,
	// path changed since it was shown: Code Internal, nothing written. A
	// selected line the fresh diff shows unchanged is skipped (as for a
	// selection from git diff -w's hunks); a selection with no changed line
	// is Code Internal too. Other refusals are GitFailed.
	StageLines(ctx context.Context, wt WorktreeID, path string, sel LineSelection, reverse bool) error

	// DiscardFile discards path's working-tree changes: git clean for an
	// untracked file or directory, else git restore to the index content.
	// The UI confirms first.
	DiscardFile(ctx context.Context, wt WorktreeID, path string, untracked bool) error

	// DiscardLines discards sel's lines of path's unstaged diff. Same
	// staleness rule as StageLines.
	DiscardLines(ctx context.Context, wt WorktreeID, path string, sel LineSelection) error

	// Commit commits the staged changes, or amends the last commit. message
	// goes over stdin (-F -), so it survives byte for byte and no editor
	// opens. Hooks run unmodified; a failing hook's output lands in the
	// error's GitStderr. Git's own refusals (nothing staged, empty message,
	// amend on an unborn HEAD) are GitFailed. Success clears the message
	// draft.
	Commit(ctx context.Context, wt WorktreeID, message string, amend bool) error

	// SetMessageDraft replaces wt's commit message draft, kept per worktree
	// session until Commit succeeds. Other windows on wt see it on their
	// next Summary; it is not pushed live.
	SetMessageDraft(ctx context.Context, wt WorktreeID, message string) error

	// DiffWorking returns path's two sides outside a commit: HEAD vs. the
	// index if staged, else the index (nothing for an untracked path) vs.
	// the working-tree file. Same rules as DiffFile. oldPath is a staged
	// rename's source.
	DiffWorking(ctx context.Context, wt WorktreeID, path, oldPath string, staged bool, opts DiffOptions) (DiffFile, error)

	// DiffFile returns path at sha's first parent (at oldPath for a rename)
	// and at sha, and git's hunks between them; a root commit has no old
	// side. Both sides are in the form git diffs them: the clean form,
	// after clean filters and line-ending conversion, for a working-tree
	// side too. Binary files and files over the size cap come without
	// content or hunks; opts.Force bypasses the size cap only.
	// opts.IgnoreWhitespace takes the hunks from git diff -w; the contents
	// stay exact.
	DiffFile(ctx context.Context, wt WorktreeID, sha, path, oldPath string, opts DiffOptions) (DiffFile, error)

	// Refresh checks wt's repo and worktree for outside changes now. Any
	// resulting "repo:changed" event follows asynchronously.
	Refresh(ctx context.Context, wt WorktreeID) error

	// ReloadLog restarts the log stream if a ref tip moved since the log
	// last finished. No-op before the first load finishes or while one runs.
	ReloadLog(ctx context.Context, wt WorktreeID) error

	// LogIndexOf finds sha's row in the loaded log, without a git call.
	// Found is false if sha is not loaded (yet).
	LogIndexOf(ctx context.Context, wt WorktreeID, sha string) (LogIndexResult, error)

	// CompareCommits lists the files that differ between a and b: a direct
	// two-revision diff, never a merge-base diff, so swapping a and b
	// mirrors every status.
	CompareCommits(ctx context.Context, wt WorktreeID, a, b string) ([]ChangedFile, error)

	// DiffRange is DiffFile for two arbitrary revisions a and b.
	DiffRange(ctx context.Context, wt WorktreeID, a, b, path, oldPath string, opts DiffOptions) (DiffFile, error)

	// DiffAgainstWorking returns path at sha (at oldPath for a rename)
	// against the working-tree file, bypassing the index (git diff <sha>).
	// Same rules as DiffFile.
	DiffAgainstWorking(ctx context.Context, wt WorktreeID, sha, path, oldPath string, opts DiffOptions) (DiffFile, error)

	// CompareWithWorking lists the files that differ between sha and wt's
	// working tree (git diff <sha>).
	CompareWithWorking(ctx context.Context, wt WorktreeID, sha string) ([]ChangedFile, error)

	// SetLogFilter starts wt's own filtered log view, or clears it for an
	// empty filter. The view belongs to the worktree, shared by every
	// window selecting it, apart from the repo's shared log; a new filter
	// kills the previous one's stream. Rows arrive like LoadLog's.
	SetLogFilter(ctx context.Context, wt WorktreeID, filter LogFilter) error

	// SearchLog finds query in wt's current view (its filtered view if one
	// is set, else the base log): a hash resolves to that commit's row
	// alone, if the view has it; otherwise every row whose message matches,
	// case-insensitively. An empty query matches nothing. Read-only: no
	// queue, no stream.
	SearchLog(ctx context.Context, wt WorktreeID, query string) (LogSearchResult, error)

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
	CheckoutCommit(ctx context.Context, wt WorktreeID, sha string) error

	// Reset moves wt's current branch, or its detached HEAD, to sha, a
	// resolved hash. Refusals are GitFailed, e.g. --soft during a merge.
	// --mixed and --hard never refuse a dirty tree or a conflict.
	Reset(ctx context.Context, wt WorktreeID, sha string, mode ResetMode) (ResetResult, error)

	// UncommittedCount is the hard-reset dialog's number: tracked paths with
	// a staged, unstaged, or unmerged change. Untracked files don't count; a
	// hard reset keeps them.
	UncommittedCount(ctx context.Context, wt WorktreeID) (int, error)

	// CherryPick applies shas onto wt's HEAD, oldest first by committer date
	// (same-second ties in topological order). A merge commit
	// among shas refuses the whole call before git runs. A stop that leaves
	// CHERRY_PICK_HEAD (a conflict or an empty result) is Code Conflicts,
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
	// flow: Code Conflicts when files conflict, else Code Stopped with git's
	// reason. A blank message, or fewer than two distinct shas, is Code
	// Internal.
	Squash(ctx context.Context, wt WorktreeID, shas []string, message string) (SquashResult, error)

	// Fixup is Squash keeping the oldest sha's message exactly.
	Fixup(ctx context.Context, wt WorktreeID, shas []string) (SquashResult, error)

	// OpenFolder starts the configured editor on wt's folder.
	OpenFolder(ctx context.Context, wt WorktreeID) error

	// OpenFile starts the configured editor on path (worktree-relative), at
	// line if line > 0.
	OpenFile(ctx context.Context, wt WorktreeID, path string, line int) error

	// OpenFolderAt starts the configured editor on path, a worktree with no
	// session (a worktree picker row other than the window's own).
	OpenFolderAt(ctx context.Context, path RepoPath) error

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

	// Jobs lists wt's repo's running jobs plus a bounded history
	// of finished ones. Updates arrive as "job:update" events.
	Jobs(ctx context.Context, wt WorktreeID) ([]JobInfo, error)

	// CancelJob cancels job id, killing its git process. No-op for an
	// unknown or finished id.
	CancelJob(ctx context.Context, wt WorktreeID, id string) error

	// Fetch fetches remote, or every remote if "", pruning gone branches,
	// as a job outside the write queues: it touches only refs and objects.
	// Returns the job id; the result arrives via "job:update".
	Fetch(ctx context.Context, wt WorktreeID, remote string) (string, error)

	// Pull fetches the current branch's upstream without a lock, then
	// merges or rebases under wt's write lock, as one job. The step equals
	// git pull --no-rebase or --rebase with the user's config: the merge
	// follows pull.ff, the rebase uses the fork point. Detached or without
	// an upstream, git's refusal surfaces as-is. A stop mid-way is Code
	// Conflicts, left for the resolve flow.
	Pull(ctx context.Context, wt WorktreeID, mode PullMode) (string, error)

	// PullWithStash is Pull with local changes in the way: after fetching,
	// it stashes them (tagged, so Summary.PullStash finds the entry),
	// integrates, and pops that exact entry only if the integrate step ends
	// cleanly. An integrate failure keeps the stash and is returned as-is;
	// a conflicting pop is GitFailed.
	PullWithStash(ctx context.Context, wt WorktreeID, mode PullMode) (string, error)

	// PopStash applies the stash entry whose commit is sha and drops it on
	// success; sha is resolved to its current stash@{n} at call time. A
	// conflicting pop keeps the entry. Code Internal if no entry has sha.
	PopStash(ctx context.Context, wt WorktreeID, sha string) error

	// DropStash deletes the stash entry sha, resolved like PopStash. The UI
	// confirms first.
	DropStash(ctx context.Context, wt WorktreeID, sha string) error

	// Push pushes wt's current branch as a job outside the write queues:
	// plainly if remote is "" (needs an upstream), else with --set-upstream
	// <remote> HEAD. A rejection is classified from --porcelain's summary
	// line: NotFastForward if the remote has unfetched commits, else
	// PushRejected. GitStderr carries git's explanation.
	Push(ctx context.Context, wt WorktreeID, remote string) (string, error)

	// PushTarget is where a plain push of the current branch would go and
	// whether it would ask first. Remote is %(push:remotename), else the
	// only remote, else origin if it exists; "" means none applies and the
	// UI must ask which remote. AutoSetUpstream is push.autoSetupRemote. A
	// detached HEAD or a bad push.autoSetupRemote value is Code GitFailed
	// with git's stderr.
	PushTarget(ctx context.Context, wt WorktreeID) (PushTarget, error)

	// ForcePush pushes with --force-with-lease --force-if-includes, never
	// plain --force. Protected-branch warnings are the UI's job. Rejections
	// are classified like Push's.
	ForcePush(ctx context.Context, wt WorktreeID) (string, error)

	// UpdateBranch fast-forwards branch from its upstream as a job, fetching
	// straight into it. Refused before the job starts: no upstream configured
	// (Code NoUpstream), or checked out in a worktree (Code GitFailed, key
	// branchCheckedOutElsewhere). A diverged branch is NotFastForward. The
	// job's Result is an UpdateBranchResult.
	UpdateBranch(ctx context.Context, wt WorktreeID, branch string) (string, error)

	// ContinueOperation runs --continue for wt's in-progress merge, rebase,
	// cherry-pick, or revert (Summary.Operation) under the write lock, with
	// GIT_EDITOR=true so no editor waits. No operation in progress is Code
	// GitFailed (key noOperation). A step that leaves the operation in
	// progress is a stop: Code Conflicts while files conflict, else Code
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

	// RenameBranch renames name to newName, validated first (Code
	// InvalidName).
	RenameBranch(ctx context.Context, wt WorktreeID, name, newName string) error

	// DeleteBranch deletes name. Without force, a branch with commits not
	// merged into its upstream, or into HEAD if it has none, is Code
	// Unmerged. A branch checked out in any worktree refuses either way
	// (GitFailed).
	DeleteBranch(ctx context.Context, wt WorktreeID, name string, force bool) error

	// RemoveWorktree removes the worktree at target in path's repo. Without
	// force, a dirty working tree is Code DirtyWorktree; force discards it.
	// A locked worktree refuses either way.
	RemoveWorktree(ctx context.Context, path RepoPath, target string, force bool) error

	// PruneWorktrees removes bookkeeping for path's repo's worktrees whose
	// directory is gone.
	PruneWorktrees(ctx context.Context, path RepoPath) error
}

// API is the whole backend: WindowAPI and the calls only the shell makes.
// Rules, enforced by tools/gencore: first parameter context.Context, last
// result error, at most one other result, and every parameter and result
// JSON-serialisable (no chan, func, or interface).
//
// The second parameter decides where the facade sends the call: a WorktreeID
// goes to the core owning that worktree, a RepoPath to the core for that path,
// anything else to the local core.
type API interface {
	WindowAPI

	// OpenWorktree validates path and returns the session's identity.
	// Idempotent: opening an already-open worktree returns the same ID.
	OpenWorktree(ctx context.Context, path RepoPath) (WorktreeInfo, error)

	// SetFocused starts (with an immediate check) or stops wt's status
	// poller as its window gains or loses focus.
	SetFocused(ctx context.Context, wt WorktreeID, focused bool) error

	// CloseWorktree ends wt's session: its poller, and the repo's watcher if
	// no other open worktree shares the repo. No-op for an unknown id.
	CloseWorktree(ctx context.Context, wt WorktreeID) error

	// SetEditorProfile sets the profile this core's OpenFolder/OpenFile use.
	// The shell calls it directly on each core when connecting: the
	// generated Forwarder would route it to the local core only.
	SetEditorProfile(ctx context.Context, profile EditorProfile) error

	// MainWorktree finds a repo's main worktree from its common git dir,
	// for when the repo's remembered worktree is gone. A bare repo with no
	// worktree left is Code NotARepo.
	MainWorktree(ctx context.Context, gitDir RepoPath) (Worktree, error)
}

// Version of a core. App is "dev" until a release build stamps it.
type Version struct {
	App      string `json:"app"`
	Protocol int    `json:"protocol"`
}

// Protocol is bumped whenever the wire format of API changes.
const Protocol = 3

// WorktreeID names a worktree session. Issued by the core that owns the
// repo; opaque to the UI. The shell and the UI name a WSL distro's
// worktree by its core's id in UNC form (\\wsl.localhost\<distro>\...),
// which routes each call to that distro's core; the shell hands the core
// back its own id.
type WorktreeID string

// RepoPath is a user-supplied path, classified by the shell as local or WSL.
type RepoPath string

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
	SHA          string    `json:"sha,omitempty"` // HEAD's commit; "" for an unborn HEAD
	EditorName   string    `json:"editorName"`    // the effective editor profile's Name
	MessageDraft string    `json:"messageDraft"`  // "" if none

	// PullStash is the commit of the newest stash entry PullWithStash made
	// that is still in the stash list, "" if none. A commit, not a
	// stash@{n} index, which shifts whenever anything else stashes.
	PullStash string `json:"pullStash"`
}

// Operation is a merge, rebase, cherry-pick or revert in progress.
type Operation string

const (
	MergeOperation      Operation = "merge"
	RebaseOperation     Operation = "rebase"
	CherryPickOperation Operation = "cherry-pick"
	RevertOperation     Operation = "revert"
)

// ResetResult is Reset's result: the HEAD commit before the reset, so the
// user can go back. "" only if HEAD was unborn.
type ResetResult struct {
	PreviousHead string `json:"previousHead"`
}

// ResetMode is git reset's mode.
type ResetMode string

const (
	ResetSoft  ResetMode = "soft"
	ResetMixed ResetMode = "mixed"
	ResetHard  ResetMode = "hard"
	ResetKeep  ResetMode = "keep"
)
