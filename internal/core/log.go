package core

// LogRowsResult is a window of the commit log loaded so far (FR-LOG-09).
type LogRowsResult struct {
	Rows       []LogCommit `json:"rows"`
	Loaded     int         `json:"loaded"` // rows loaded so far; grows until Done
	Done       bool        `json:"done"`
	Error      *Error      `json:"error,omitempty"`
	Generation int         `json:"generation"` // bumps on ReloadLog; a store sees this change and drops its row cache
}

// LogCommit is one commit-log row (FR-LOG-01).
type LogCommit struct {
	SHA          string     `json:"sha"`
	ShortSHA     string     `json:"shortSha"`
	Author       string     `json:"author"`
	AuthorAt     int64      `json:"authorAt"` // unix seconds
	Subject      string     `json:"subject"`
	Refs         []RefLabel `json:"refs,omitempty"`
	CheckedOutIn []string   `json:"checkedOutIn,omitempty"` // worktree paths (FR-LOG-10)
	Graph        GraphRow   `json:"graph"`
}

// RefLabel is one decoration on a commit (FR-LOG-01). Name is the short
// display form ("main", "origin/main", "v1.0", "HEAD"); Kind disambiguates
// what --decorate=full's full ref path said it was.
type RefLabel struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "head" | "branch" | "remote" | "tag"
}

// GraphRow is a commit's lane data (ADR-5), the JSON shape of graph.Row.
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
// changed files vs. its first parent (FR-LOG-03).
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

// ChangedFile is one file touched by a commit.
type ChangedFile struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"` // rename/copy source
	Status  string `json:"status"`            // "A" | "M" | "D" | "R" | "C" | "T" | "U"
}
