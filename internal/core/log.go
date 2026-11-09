package core

import "context"

// Log is the repo's commits and what they changed: the streamed log with its
// graph and filters, a commit's details, comparisons, and diffs of committed
// files.
type Log interface {
	// LoadLog starts loading the repo's commit log and graph unless already
	// loading or loaded (idempotent per repo). Returns once the load has
	// started; rows are read via LogRows and announced by "log:progress".
	LoadLog(ctx context.Context, wt WorktreeID) error

	// LogRows returns rows [from, to) of what has loaded so far, clamped to
	// [0, loaded). Error (and Done) is set if the load failed.
	LogRows(ctx context.Context, wt WorktreeID, from, to int) (LogRowsResult, error)

	// ReloadLog restarts the log stream if a ref tip moved since the log
	// last finished. No-op before the first load finishes or while one runs.
	ReloadLog(ctx context.Context, wt WorktreeID) error

	// LogIndexOf finds sha's row in the loaded log, without a git call.
	// Found is false if sha is not loaded (yet).
	LogIndexOf(ctx context.Context, wt WorktreeID, sha SHA) (LogIndexResult, error)

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

	// CommitDetails returns one commit's full message, committer, parents,
	// and changed files vs. its first parent.
	CommitDetails(ctx context.Context, wt WorktreeID, sha SHA) (CommitDetails, error)

	// CompareCommits lists the files that differ between a and b: a direct
	// two-revision diff, never a merge-base diff, so swapping a and b
	// mirrors every status.
	CompareCommits(ctx context.Context, wt WorktreeID, a, b SHA) ([]ChangedFile, error)

	// CompareWithWorking lists the files that differ between sha and wt's
	// working tree (git diff <sha>).
	CompareWithWorking(ctx context.Context, wt WorktreeID, sha SHA) ([]ChangedFile, error)

	// DiffFile returns path at sha's first parent (at oldPath for a rename)
	// and at sha, and git's hunks between them; a root commit has no old
	// side. Both sides are in the form git diffs them: the clean form,
	// after clean filters and line-ending conversion, for a working-tree
	// side too. Binary files and files over the size cap come without
	// content or hunks; opts.Force bypasses the size cap only.
	// opts.IgnoreWhitespace takes the hunks from git diff -w; the contents
	// stay exact.
	DiffFile(ctx context.Context, wt WorktreeID, sha SHA, path, oldPath string, opts DiffOptions) (DiffFile, error)

	// DiffRange is DiffFile for two arbitrary revisions a and b.
	DiffRange(ctx context.Context, wt WorktreeID, a, b SHA, path, oldPath string, opts DiffOptions) (DiffFile, error)

	// DiffAgainstWorking returns path at sha (at oldPath for a rename)
	// against the working-tree file, bypassing the index (git diff <sha>).
	// Same rules as DiffFile.
	DiffAgainstWorking(ctx context.Context, wt WorktreeID, sha SHA, path, oldPath string, opts DiffOptions) (DiffFile, error)
}

// LogRowsResult is a window of the commit log loaded so far.
type LogRowsResult struct {
	Rows       []LogCommit `json:"rows"`
	Loaded     int         `json:"loaded"` // rows loaded so far; grows until Done
	Done       bool        `json:"done"`
	Error      *Error      `json:"error,omitempty"`
	Generation int         `json:"generation"` // bumps on ReloadLog; a store sees this change and drops its row cache
}

// LogCommit is one commit-log row.
type LogCommit struct {
	SHA          SHA        `json:"sha"`
	ShortSHA     string     `json:"shortSha"`
	Author       string     `json:"author"`
	AuthorAt     int64      `json:"authorAt"` // unix seconds
	Subject      string     `json:"subject"`
	Refs         []RefLabel `json:"refs,omitempty"`
	CheckedOutIn []string   `json:"checkedOutIn,omitempty"` // paths of worktrees whose HEAD is this commit
	Graph        GraphRow   `json:"graph"`
}

// RefLabel is one decoration on a commit. Name is the short display form
// ("main", "origin/main", "v1.0", "HEAD").
type RefLabel struct {
	Name string  `json:"name"`
	Kind RefKind `json:"kind"`
}

// RefKind is what a RefLabel names; "" for a ref outside refs/heads,
// refs/remotes and refs/tags.
type RefKind string

const (
	HeadRef   RefKind = "head"
	BranchRef RefKind = "branch"
	RemoteRef RefKind = "remote"
	TagRef    RefKind = "tag"
)

// GraphRow is a commit's lane data, the JSON shape of graph.Row.
type GraphRow struct {
	Lane     int         `json:"lane"`
	Colour   int         `json:"colour"`
	Edges    []GraphEdge `json:"edges"`
	Overflow int         `json:"overflow,omitempty"`
}

// GraphEdge is the JSON shape of graph.Edge.
type GraphEdge struct {
	Lane   int      `json:"lane"`
	Kind   EdgeKind `json:"kind"`
	Colour int      `json:"colour"`
}

// EdgeKind is the JSON shape of graph.EdgeKind.
type EdgeKind string

const (
	ThroughEdge EdgeKind = "through"
	InEdge      EdgeKind = "in"  // from Lane into the commit
	OutEdge     EdgeKind = "out" // from the commit to Lane
)

// CommitDetails is one commit's full message, committer, parents, and
// changed files vs. its first parent.
type CommitDetails struct {
	SHA            SHA           `json:"sha"`
	Parents        []SHA         `json:"parents"`
	Author         string        `json:"author"`
	AuthorEmail    string        `json:"authorEmail"`
	AuthorAt       int64         `json:"authorAt"`
	Committer      string        `json:"committer"`
	CommitterEmail string        `json:"committerEmail"`
	CommitterAt    int64         `json:"committerAt"`
	Body           string        `json:"body"` // full message, subject line included
	Files          []ChangedFile `json:"files"`
}

// LogFilter is the log's combined filter: every non-empty field narrows the
// result (AND).
type LogFilter struct {
	Author   string    `json:"author,omitempty"`   // author name or email
	Branches []RefName `json:"branches,omitempty"` // branches or tags; empty = every branch, like the unfiltered log
	Path     string    `json:"path,omitempty"`     // file or folder path
}

// Empty reports whether f narrows nothing; SetLogFilter with an empty
// filter clears the filtered view.
func (f LogFilter) Empty() bool {
	return f.Author == "" && len(f.Branches) == 0 && f.Path == ""
}

// Flat reports whether f's result has no graph: an author search walks
// history out of topological order.
func (f LogFilter) Flat() bool {
	return f.Author != ""
}

// LogSearchResult is SearchLog's result: the matching rows' indices, in log
// order, in the view Filtered names (the worktree's filtered view or the
// repo's shared log) at Generation. The two views number their generations
// separately, so telling a stale answer needs both fields.
type LogSearchResult struct {
	Indices    []int `json:"indices,omitempty"`
	Generation int   `json:"generation"`
	Filtered   bool  `json:"filtered"`
}

// LogIndexResult is LogIndexOf's result.
type LogIndexResult struct {
	Index int  `json:"index"`
	Found bool `json:"found"`
}

// LogProgress is "log:progress"'s payload. Filtered tells the repo-wide
// unfiltered log apart from a worktree's own filtered view: one worktree
// can have both streams running at once, and every window on it sees the
// same progress. Generation is that stream's own LogRowsResult.Generation:
// a reload that keeps the commit count still tells a window its cached
// rows are stale. Error is set on the done progress of a load that failed.
type LogProgress struct {
	Loaded     int    `json:"loaded"`
	Done       bool   `json:"done"`
	Filtered   bool   `json:"filtered"`
	Generation int    `json:"generation"`
	Error      *Error `json:"error,omitempty"`
}
