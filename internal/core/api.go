// Package core is the backend contract: one interface, JSON-serialisable
// DTOs, one error type, and events (TR-28, ADR-3).
//
// Implemented in-process by internal/services and over JSON-RPC by
// internal/remote.Client. The shell binds the generated app.Facade, which
// forwards each call to the core that owns the window's worktree.
package core

import "context"

//go:generate go run ../../tools/gencore

// API is the whole backend. Rules, enforced by tools/gencore: first parameter
// context.Context, last result error, at most one other result, and every
// parameter and result JSON-serialisable (no chan, func, or interface).
//
// The second parameter decides where the facade sends the call: a WorktreeID
// goes to the core owning that worktree, a RepoPath to the core for that path,
// anything else to the local core.
type API interface {
	// Version identifies the core; the shell compares Protocol on connect.
	Version(ctx context.Context) (Version, error)

	// OpenWorktree validates path and returns the session's identity
	// (FR-REPO-01, FR-REPO-05). Idempotent: opening an already-open
	// worktree returns the same ID.
	OpenWorktree(ctx context.Context, path RepoPath) (WorktreeInfo, error)

	// Summary is FR-REPO-03's "always show" panel minus ahead/behind and the
	// refs lists. Cheap and repeatable — the watcher calls it again on every
	// change.
	Summary(ctx context.Context, wt WorktreeID) (Summary, error)

	// Refs lists every local branch, every remote's tracking branches, and
	// every tag (FR-LOG-01's sidebar half; the per-commit row labels come
	// with the log rows). Not streamed, not cached in session.Repo — unlike
	// LoadLog/LogRows: one for-each-ref call costs the same single process
	// spawn no matter how many branches exist, so recomputing on every
	// call is as cheap as a cache would be.
	Refs(ctx context.Context, wt WorktreeID) (Refs, error)

	// CommandLog returns the buffered entries so far (FR-UX-02): a window
	// that opens the panel after some commands already ran does not miss
	// them; new ones after that arrive via the "command" event.
	CommandLog(ctx context.Context, wt WorktreeID) ([]CommandLogEntry, error)

	// LoadLog starts loading wt's repo's commit log and graph if not
	// already loading or loaded (idempotent per repo, ADR-9's shared Repo
	// session). Returns once the load has started, not once it finishes;
	// rows arrive incrementally, readable via LogRows and announced by
	// "log:progress" events.
	LoadLog(ctx context.Context, wt WorktreeID) error

	// LogRows returns rows [from, to) of what has loaded so far
	// (FR-LOG-09): the window is clamped to [0, loaded), never an error for
	// a window past the current end. Error is set (Done also true) if the
	// load itself failed (FR-UX-01).
	LogRows(ctx context.Context, wt WorktreeID, from, to int) (LogRowsResult, error)

	// CommitDetails returns one commit's full message, committer, parents,
	// and changed files vs. its first parent (FR-LOG-03).
	CommitDetails(ctx context.Context, wt WorktreeID, sha string) (CommitDetails, error)

	// Status lists wt's current uncommitted changes, grouped (FR-CHG-01):
	// staged, unstaged, untracked, conflicted. A file changed both ways
	// (git's own "MM"-shaped record) appears once in Staged and once in
	// Unstaged, from that one record's two independent halves.
	Status(ctx context.Context, wt WorktreeID) (ChangedFiles, error)

	// StageFile / UnstageFile move one path between the working tree and the
	// index in wt (FR-CHG-02): git add for staging (a modified tracked file
	// or an untracked one alike — no intent-to-add step needed at file
	// granularity), git restore --staged for unstaging. A refusal (the path
	// no longer matches anything, a race with an outside change) is git's
	// own, passed through as plain GitFailed — no pre-check.
	StageFile(ctx context.Context, wt WorktreeID, path string) error
	UnstageFile(ctx context.Context, wt WorktreeID, path string) error

	// StageAll / UnstageAll act on every changed file at once (FR-CHG-02):
	// git add -A for staging (tracked and untracked alike); git restore
	// --staged -- . for unstaging, chosen over a bare git reset for an
	// identical result with no incidental human-readable stdout.
	StageAll(ctx context.Context, wt WorktreeID) error
	UnstageAll(ctx context.Context, wt WorktreeID) error

	// DiffWorking returns path's two-sided content outside a commit
	// (FR-CHG-04), a sibling of DiffFile/DiffRange: staged compares HEAD to
	// the index — both git objects. !staged compares the index (or nothing,
	// if path has no index entry at all — untracked) to the literal
	// working-tree file, which is not a git object until staged. Binary,
	// size-cap, and UTF-8 rules match DiffFile exactly either way. oldPath
	// is the rename source for a staged rename.
	DiffWorking(ctx context.Context, wt WorktreeID, path, oldPath string, staged, force bool) (DiffFile, error)

	// DiffFile returns path's content on both sides of sha (FR-LOG-04): at
	// sha^ (oldPath if the file was renamed into path, else path) and at sha
	// itself — first-parent semantics, matching CommitDetails: a merge
	// diffs against its first parent only, and sha's root commit (no
	// parent) yields "missing" on the old side, not an error. Binary files
	// (FR-DIFF-03) and files over the size cap (FR-DIFF-04) come back
	// without content; force bypasses the size cap only — a binary result
	// never carries content regardless of force.
	DiffFile(ctx context.Context, wt WorktreeID, sha, path, oldPath string, force bool) (DiffFile, error)

	// Refresh forces an immediate check of wt's repo and worktree for
	// outside changes (FR-REPO-04: manual refresh). Does not wait for any
	// resulting "repo:changed" event — those follow the normal path if
	// something actually differs.
	Refresh(ctx context.Context, wt WorktreeID) error

	// SetFocused tells wt's session whether its window currently has focus
	// (FR-REPO-04's "immediately on focus"; QA-PERF-06's CPU bound):
	// focused=true runs an immediate poller check and resumes its timer;
	// focused=false stops the timer.
	SetFocused(ctx context.Context, wt WorktreeID, focused bool) error

	// CloseWorktree ends wt's session (ADR-9): stops its poller, and — if
	// no other open worktree shares its repo — the repo's watcher too. A
	// no-op for an unknown id.
	CloseWorktree(ctx context.Context, wt WorktreeID) error

	// ReloadLog re-checks wt's repo's ref tips (cheap: for-each-ref, not a
	// commit walk) against the snapshot taken when the log last finished,
	// and restarts the log/graph stream only if they differ. No-op if
	// nothing has finished loading yet (call LoadLog first) or a load is
	// already running.
	ReloadLog(ctx context.Context, wt WorktreeID) error

	// LogIndexOf finds sha's row index in wt's repo's currently loaded log
	// (FR-LOG-01: clicking a ref in the sidebar selects and scrolls to its
	// commit). A linear scan over already-streamed rows, not a git call.
	// Found is false if sha is not, or not yet, among the loaded rows.
	LogIndexOf(ctx context.Context, wt WorktreeID, sha string) (LogIndexResult, error)

	// CompareCommits lists the files that differ between a and b (FR-CMP-01):
	// a direct two-revision diff (a..b), never a merge-base diff (a...b) —
	// only the two-revision form makes swapping a and b mirror every file's
	// status exactly (FR-CMP-03). Same ChangedFile shape as CommitDetails, so
	// a consumer already knows how to render it.
	CompareCommits(ctx context.Context, wt WorktreeID, a, b string) ([]ChangedFile, error)

	// DiffRange returns path's content at two arbitrary revisions a and b
	// (FR-CMP-02): DiffFile's counterpart for two independently chosen
	// commits instead of one commit vs its first parent. Same result DTO,
	// same binary/size-cap rules — force bypasses the size cap only, a binary
	// result never carries content regardless of force.
	DiffRange(ctx context.Context, wt WorktreeID, a, b, path, oldPath string, force bool) (DiffFile, error)

	// SetLogFilter starts (filter non-empty) or clears (filter.Empty()) wt's
	// own filtered view of the log (FR-LOG-05..08). A filtered view is this
	// window's private state, independent of wt's repo's shared unfiltered
	// cache. A new non-empty filter supersedes whatever filtered view was
	// already active or loading, killing its stream. Returns once the view
	// has started changing, not once it finishes; rows arrive incrementally,
	// readable via LogRows and announced by "log:progress" events exactly
	// like LoadLog.
	SetLogFilter(ctx context.Context, wt WorktreeID, filter LogFilter) error

	// CreateBranch creates a new branch named name at start — a commit, an
	// existing branch, or any other revision git itself resolves (FR-BR-01;
	// "from any commit" or "from HEAD" both just supply start). name is
	// validated (syntax) before any mutating git call, so an invalid name
	// comes back as Code InvalidName without ever touching the worktree; a
	// duplicate name is git's own refusal (GitFailed, "a branch named ...
	// already exists") — FR-BR-01: "rejected with a clear message".
	// checkout switches this worktree to the new branch in the same atomic
	// git call (a checkout failure then creates no orphan branch) — a
	// refusal (local changes in the way) comes back as Code DirtyWorktree,
	// same as Checkout/CheckoutCommit's own refusal.
	CreateBranch(ctx context.Context, wt WorktreeID, name, start string, checkout bool) error

	// Checkout switches this worktree to branch: an existing local branch, or
	// — via git's own DWIM, no app-side branch list needed — a
	// remote-tracking branch with no local branch of that name yet, which
	// git turns into a new local branch tracking it (FR-BR-05). A refusal is
	// git's own, returned as-is: Code DirtyWorktree for local changes in the
	// way (a tracked file's modification, or an untracked file, that
	// checkout would overwrite), plain GitFailed for anything else (an
	// ambiguous name, an invalid/nonexistent branch, or the branch already
	// checked out in another worktree — its own message names the
	// worktree's path). No pre-check, no stash: "if git refuses... show why"
	// (FR-BR-05) is git's own message, not a guess this app makes first.
	Checkout(ctx context.Context, wt WorktreeID, branch string) error

	// CheckoutCommit detaches this worktree's HEAD at sha (FR-BR-08). Same
	// DirtyWorktree/GitFailed split as Checkout. Unlike Checkout, never
	// refused for being "already checked out": a detached commit may be
	// checked out in several worktrees at once.
	CheckoutCommit(ctx context.Context, wt WorktreeID, sha string) error

	// Reset moves wt's current branch — or, while detached, HEAD directly;
	// git treats both the same (FR-BR-02) — to sha. sha is always an
	// already-resolved commit hash (a LogCommit.SHA or Summary.SHA), never
	// free user text: no name validation, no --end-of-options, no --
	// (reset rejects the former outright and the latter silently
	// reinterprets the target as a pathspec — neither is needed for a
	// value that structurally cannot start with "-"). A refusal is
	// returned as-is, plain GitFailed: an invalid/nonexistent sha, or
	// --soft's own "cannot do a soft reset in the middle of a merge"
	// during any unresolved conflict — both are exit 128 with no other
	// machine-readable signal to split them by, unlike Checkout's
	// exit-1-vs-128 split. --mixed/--hard are never refused for a dirty
	// tree or a conflict in progress, mid-merge included — git's own
	// behaviour, not pre-blocked, the same stance Checkout takes.
	Reset(ctx context.Context, wt WorktreeID, sha string, mode ResetMode) (ResetResult, error)

	// UncommittedCount is FR-BR-03's confirmation-dialog number: how many
	// tracked paths carry an uncommitted change right now (staged,
	// unstaged, or unmerged) — never untracked files, which an ordinary
	// hard reset does not delete.
	UncommittedCount(ctx context.Context, wt WorktreeID) (int, error)

	// CherryPick applies shas onto wt's current HEAD (branch or detached,
	// FR-WT-05 frames this as worktree-scoped, not branch-scoped) — one
	// commit or several (FR-CP-01). shas may be given in any order; the
	// service re-sorts them oldest-first itself before calling git
	// (FR-CP-02: "whatever the selection order"; ties in commit time keep
	// the caller's order reversed, so pass log order, newest first).
	// Refuses the whole call, before running git at all, if any of shas is
	// a merge commit (FR-CP-05): git's own per-commit refusal cannot
	// deliver "nothing changed" for a batch, so this app-level check is
	// what does. A stop with git's own CHERRY_PICK_HEAD state file left
	// behind (a real content conflict, or an already-applied/empty result)
	// comes back as Code Conflicts, left in place for a later
	// abort/continue/skip flow. Any other refusal (a dirty file in the
	// way, another operation already running, a nonexistent sha) is git's
	// own, passed through as plain GitFailed (FR-CP-04's "show why").
	CherryPick(ctx context.Context, wt WorktreeID, shas []string) error

	// Revert is CherryPick's undo counterpart (FR-RV-01): applies shas
	// newest-first (FR-RV-02, load-bearing the same way as FR-CP-02's
	// ordering); every other rule — the merge pre-check,
	// refusal classification, Code Conflicts on a stop — is identical
	// (FR-RV-03: "handled as for cherry-pick").
	Revert(ctx context.Context, wt WorktreeID, shas []string) error

	// SetEditorProfile installs the editor profile this core uses for every
	// OpenFolder/OpenFile call it serves (TR-13: pushed by the shell "on
	// connect"). Never called from the frontend: internal/app calls it
	// directly on the concrete core.API value it already holds at the
	// moment of connecting, not through the generated Facade (whose own
	// forwarder for this method would always resolve to the local core,
	// since neither of SetEditorProfile's parameters is a routing key).
	SetEditorProfile(ctx context.Context, profile EditorProfile) error

	// OpenFolder starts wt's configured editor on its own worktree folder
	// (FR-ED-01, FR-ED-03).
	OpenFolder(ctx context.Context, wt WorktreeID) error

	// OpenFile starts wt's configured editor on path, at line if line > 0
	// (FR-ED-04). path is worktree-relative, resolved against wt's own
	// toplevel (FR-ED-02): the commit-details, compare, and compare-diff
	// file lists, and the diff pane's toolbar.
	OpenFile(ctx context.Context, wt WorktreeID, path string, line int) error

	// OpenFolderAt starts the configured editor on path (FR-WT-04) with no
	// worktree session: a panel row nobody has selected has no WorktreeID
	// to route by, so it takes a path like ListWorktrees and AddWorktree.
	OpenFolderAt(ctx context.Context, path RepoPath) error

	// ListWorktrees lists every worktree of path's repo (FR-WT-01): path,
	// branch or detached commit, clean/dirty, main/locked/missing. Stateless
	// (session.Manager.Resolve) — resolves path, then runs git directly; no
	// session.Worktree is minted, so expanding a repo's panel node starts no
	// poller. Cheap and repeatable, same "no cache" precedent as Refs and
	// Summary: a repo's worktree count is small, unlike its commit count.
	ListWorktrees(ctx context.Context, path RepoPath) ([]Worktree, error)

	// ListBranches lists path's repo's local branches and which worktree, if
	// any, has each checked out (FR-WT-13's "and their branches", for a repo
	// with no worktree open anywhere yet — Refs needs an already-open
	// WorktreeID and carries upstream/ahead-behind data no panel action needs).
	// Stateless, same reasoning as ListWorktrees.
	ListBranches(ctx context.Context, path RepoPath) ([]BranchSummary, error)

	// AddWorktree creates a worktree at target, from an existing local branch
	// (newBranch false, start ignored) or a new one (newBranch true: branch
	// names the branch to create, start is the branch/commit to start it from
	// — mirrors CreateBranch's own name/start shape). target must not exist,
	// or must be empty (FR-WT-03): git's own refusal, not pre-checked.
	// newBranch's name is validated (Code InvalidName) before any mutating
	// call — required here, unlike CreateBranch's switch -c, because
	// worktree add -b's own argument is not protected by --.
	AddWorktree(ctx context.Context, path RepoPath, target, branch, start string, newBranch bool) (Worktree, error)

	// Jobs lists wt's repo's jobs (JobService, components.md): every
	// queued/running one, plus a bounded history of recently finished ones.
	// Cheap and repeatable, same reasoning as Summary (spec 05) — the strip
	// re-polls it on mount and otherwise lives off "job:update" events.
	Jobs(ctx context.Context, wt WorktreeID) ([]JobInfo, error)

	// CancelJob cancels id if it is still running (FR-SYNC-07, QA-RESP-02):
	// cancels the job's context, which — for any job whose fn runs a git
	// call through it — kills the child process (TR-11). A no-op for an
	// unknown or already-finished id, same convention as CloseWorktree
	// (spec 12).
	CancelJob(ctx context.Context, wt WorktreeID, id string) error

	// Fetch fetches wt's repo from remote, or every remote when remote == ""
	// (git fetch --prune --progress <remote>|--all), pruning remote-tracking
	// branches gone on the far side. Runs as a job, outside the write queues:
	// touches only refs/objects, never this worktree's index or files, so it
	// never blocks and is never blocked by a concurrent stage/commit/checkout.
	// Returns the new job's id; progress and the result arrive via
	// "job:update", not this call's return.
	Fetch(ctx context.Context, wt WorktreeID, remote string) (string, error)

	// Pull fetches wt's current branch's upstream, then integrates it by
	// merge or rebase, as one job: the fetch runs first, holding no lock,
	// then wt.Lock()s only for the local integration step. Requires an
	// attached branch with an upstream configured; while detached, or with
	// none, git's own plain refusal surfaces as-is. A stop mid-way is Code
	// Conflicts (MERGE_HEAD, or rebase-merge/rebase-apply, left behind), left
	// in place for the resolve flow, same convention as CherryPick/Revert.
	Pull(ctx context.Context, wt WorktreeID, mode PullMode) (string, error)

	// Push pushes wt's current branch (git push --progress), as a job,
	// outside the write queues. remote == "" pushes plainly (the branch must
	// already have an upstream, or this is git's own plain refusal); a
	// non-empty remote instead runs git push --progress --set-upstream
	// <remote> HEAD. A rejection is classified from the push's own
	// --porcelain summary line on stdout: Code NotFastForward for the
	// "fetch/integrate first" class of reasons, Code PushRejected for every
	// other explained one; GitStderr carries git's own explanation unchanged
	// either way.
	Push(ctx context.Context, wt WorktreeID, remote string) (string, error)

	// ForcePush force-pushes wt's current branch in the one safe form: git
	// push --progress --force-with-lease --force-if-includes — bare
	// --force-with-lease, never the =<ref>:<expect> form, which would
	// silently disable --force-if-includes; plain --force is never offered.
	// No app-level refusal for a protected branch: the confirmation and its
	// warning are entirely the UI's job. Rejection classified the same way
	// as Push's.
	ForcePush(ctx context.Context, wt WorktreeID) (string, error)
}

// Version of a core. App is "dev" until a release build stamps it.
type Version struct {
	App      string `json:"app"`
	Protocol int    `json:"protocol"`
}

// Protocol is bumped whenever the wire format of API changes.
const Protocol = 1

// WorktreeID names a worktree session (ADR-9). Issued by the core that owns
// the repo; opaque to the UI.
type WorktreeID string

// RepoPath is a user-supplied path, classified by the shell as local or WSL.
type RepoPath string

// WorktreeInfo identifies an opened worktree session.
type WorktreeInfo struct {
	ID      WorktreeID `json:"id"`
	Path    string     `json:"path"`
	RepoKey string     `json:"repoKey"` // common git dir; shell bookkeeping (Registry.SetRepo), opaque to the UI
}

// Summary is the worktree's always-shown branch/state panel (FR-REPO-03).
type Summary struct {
	Path      string `json:"path"`
	Branch    string `json:"branch"` // "" when Detached
	Detached  bool   `json:"detached"`
	Operation string `json:"operation"` // "", "merge", "rebase", "cherry-pick", "revert"

	// SHA is HEAD's own resolved commit — "" only for an unborn HEAD (a
	// fresh repo with no commits yet, QA-REL-02), never an error. Lets
	// "create branch here" (FR-BR-08, attached or detached) and a future
	// "new branch from HEAD" toolbar action know a start point without
	// depending on the log having that commit loaded.
	SHA string `json:"sha,omitempty"`

	// EditorName is the effective editor profile's Name (FR-ED-05), so the
	// UI can label its "Open in {name}" button without a second round trip.
	EditorName string `json:"editorName"`
}

// ResetResult is Reset's result: the HEAD commit before the reset, so the
// user can go back (FR-BR-04). "" only if HEAD was unborn.
type ResetResult struct {
	PreviousHead string `json:"previousHead"`
}

// ResetMode is FR-BR-02's three choices, one explanatory line each in the
// dialog. The fourth mode, --keep (FR-BR-07), is not offered yet.
type ResetMode string

const (
	ResetSoft  ResetMode = "soft"
	ResetMixed ResetMode = "mixed"
	ResetHard  ResetMode = "hard"
)
