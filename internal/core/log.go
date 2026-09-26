package core

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
	SHA          string     `json:"sha"`
	ShortSHA     string     `json:"shortSha"`
	Author       string     `json:"author"`
	AuthorAt     int64      `json:"authorAt"` // unix seconds
	Subject      string     `json:"subject"`
	Refs         []RefLabel `json:"refs,omitempty"`
	CheckedOutIn []string   `json:"checkedOutIn,omitempty"` // paths of worktrees whose HEAD is this commit
	Graph        GraphRow   `json:"graph"`
}

// RefLabel is one decoration on a commit. Name is the short
// display form ("main", "origin/main", "v1.0", "HEAD"); Kind disambiguates
// what --decorate=full's full ref path said it was.
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
	SHA            string        `json:"sha"`
	Parents        []string      `json:"parents"`
	Author         string        `json:"author"`
	AuthorEmail    string        `json:"authorEmail"`
	AuthorAt       int64         `json:"authorAt"`
	Committer      string        `json:"committer"`
	CommitterEmail string        `json:"committerEmail"`
	CommitterAt    int64         `json:"committerAt"`
	Body           string        `json:"body"` // full message, subject line included
	Files          []ChangedFile `json:"files"`
}

// ChangedFile is one file touched by a commit, or one entry in a worktree's
// current changes.
type ChangedFile struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // rename/copy source
	// Status is "A" | "M" | "D" | "R" | "C" | "T" | "U" for a commit's own
	// file; "?" for an untracked entry, or a two-letter unmerged code (e.g.
	// "UU", "AU", "DD") for a conflicted entry.
	Status string `json:"status"`
}

// LogFilter is the log's combined filter: every non-empty field narrows the
// result (AND).
type LogFilter struct {
	Author   string   `json:"author,omitempty"`   // author name or email
	Branches []string `json:"branches,omitempty"` // full ref names of branches or tags (refs/heads/x, refs/tags/x): a short one can name either; empty = every branch, like the unfiltered log
	Path     string   `json:"path,omitempty"`     // file or folder path
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
// order, in the view Generation and Filtered identify together (as
// LogProgress's own Filtered does): the worktree's filtered view, or the
// repo's shared log. The two views number their generations separately, so
// a frontend must check both to tell a stale answer from a fresh one.
type LogSearchResult struct {
	Indices    []int `json:"indices,omitempty"`
	Generation int   `json:"generation"`
	Filtered   bool  `json:"filtered"`
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
