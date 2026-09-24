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
	Merge        bool       `json:"merge"` // two or more parents
}

// RefLabel is one decoration on a commit. Name is the short
// display form ("main", "origin/main", "v1.0", "HEAD"); Kind disambiguates
// what --decorate=full's full ref path said it was.
type RefLabel struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "head" | "branch" | "remote" | "tag"
}

// GraphRow is a commit's lane data, the JSON shape of graph.Row.
type GraphRow struct {
	Lane     int         `json:"lane"`
	Colour   int         `json:"colour"`
	Edges    []GraphEdge `json:"edges"`
	Overflow int         `json:"overflow,omitempty"`
}

// GraphEdge is the JSON shape of graph.Edge.
type GraphEdge struct {
	Lane int    `json:"lane"`
	Kind string `json:"kind"` // "start" | "continue" | "end"
}

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
// result (AND). Query, if it resolves to an existing commit's hash,
// short-circuits every other field.
type LogFilter struct {
	Query    string   `json:"query,omitempty"`    // message text (subject+body), or a commit hash
	Author   string   `json:"author,omitempty"`   // author name or email
	Branches []string `json:"branches,omitempty"` // short ref names; empty = every branch, like the unfiltered log
	Path     string   `json:"path,omitempty"`     // file or folder path
}

// Empty reports whether f narrows nothing; SetLogFilter with an empty
// filter clears the filtered view.
func (f LogFilter) Empty() bool {
	return f.Query == "" && f.Author == "" && len(f.Branches) == 0 && f.Path == ""
}

// Flat reports whether f's result has no graph: a text/author search walks
// history out of topological order.
func (f LogFilter) Flat() bool {
	return f.Query != "" || f.Author != ""
}

// LogProgress is "log:progress"'s payload. Filtered tells the repo-wide
// unfiltered log apart from a window's own filtered view: one window can
// have both streams running at once. Generation is that stream's own
// LogRowsResult.Generation (the repo-wide log's, or the window's own
// filtered view's): a reload that keeps the commit count still tells a
// window its cached rows are stale.
type LogProgress struct {
	Loaded     int  `json:"loaded"`
	Done       bool `json:"done"`
	Filtered   bool `json:"filtered"`
	Generation int  `json:"generation"`
}
