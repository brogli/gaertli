package services

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/graph"
	"github.com/brogli/nebegit/internal/session"
)

// largeRepoRows is the loaded-row threshold above which streamLog writes
// git's own commit-graph file once. A var so tests can lower it.
var largeRepoRows = 20_000

const progressInterval = 100 * time.Millisecond

// logTimeout bounds the log stream: the runner's 30s default is too short
// for a large repo.
const logTimeout = 10 * time.Minute

// LogFormat is the git log record layout LoadLog streams; exported so
// tools/bench measures exactly what the app runs.
const LogFormat = "%H%x1f%h%x1f%P%x1f%an%x1f%at%x1f%D%x1f%s%x00"

// LoadLog implements core.API.
func (c *Core) LoadLog(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	gen, ok := wt.Repo.LogBeginLoad()
	if !ok {
		return nil
	}
	go c.streamLog(wt, gen)
	return nil
}

// LogRows implements core.API.
func (c *Core) LogRows(ctx context.Context, id core.WorktreeID, from, to int) (core.LogRowsResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.LogRowsResult{}, err
	}
	view := wt.LogSnapshot(from, to)
	return core.LogRowsResult{
		Rows: view.Rows, Loaded: view.Loaded, Done: view.Done, Error: view.Err,
		Generation: view.Generation, Flat: view.Flat,
	}, nil
}

// ReloadLog implements core.API.
func (c *Core) ReloadLog(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if !wt.Repo.LogReady() {
		return nil
	}
	tips, err := refTips(ctx, wt.Runner, wt.Path)
	if err != nil {
		return err
	}
	if maps.Equal(tips, wt.Repo.LogTips()) {
		return nil // no ref moved
	}
	gen, ok := wt.Repo.LogBeginReload()
	if !ok {
		return nil // a load is already running; it will record fresh tips itself
	}
	go c.streamLog(wt, gen)
	return nil
}

// LogIndexOf implements core.API.
func (c *Core) LogIndexOf(ctx context.Context, id core.WorktreeID, sha string) (core.LogIndexResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.LogIndexResult{}, err
	}
	index, found := wt.Repo.LogIndexOf(sha)
	return core.LogIndexResult{Index: index, Found: found}, nil
}

// streamLog runs in its own goroutine, detached from LoadLog's ctx (done as
// soon as LoadLog returns) — a background load outlives one RPC call. gen
// guards every LogAppend/LogFinish against a reload that superseded it.
func (c *Core) streamLog(wt *session.Worktree, gen int) {
	repo := wt.Repo
	progress := func(loaded int, done bool) { c.emitLogProgress(repo.Key, loaded, done) }
	loaded, logErr := streamCommits(context.Background(), wt, LogArgs(core.LogFilter{}), false, true,
		func(commit core.LogCommit) { repo.LogAppend(gen, commit) }, progress)

	var tips map[string]string
	if logErr == nil {
		// best-effort: a failure here just means the next ReloadLog always
		// refreshes, rather than failing an otherwise-successful load.
		tips, _ = refTips(context.Background(), wt.Runner, wt.Path)
	}
	repo.LogFinish(gen, logErr, tips)
	progress(loaded, true)
	if logErr == nil {
		writeCommitGraphIfLarge(wt, loaded) // after the done signal: never delays the UI
	}
}

// streamCommits runs one git log call with args, parses every record into a
// LogCommit (with its graph row unless flat), and hands each to appendRow;
// progress fires for the first row and then every progressInterval, never
// with done. Returns the row count and the call's error, if any.
func streamCommits(ctx context.Context, wt *session.Worktree, args []string, flat, background bool,
	appendRow func(core.LogCommit), progress func(loaded int, done bool)) (loaded int, err *core.Error) {
	checkedOut, ckErr := worktreeCheckouts(wt.Runner, wt.Path, background)
	if ckErr != nil {
		errors.As(ckErr, &err)
		return 0, err
	}

	layout := graph.NewLayout()
	lastEmit := time.Time{}
	_, runErr := wt.Runner.Stream(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true, Background: background, Timeout: logTimeout, Args: args,
	}, func(record []byte) error {
		commit, parents, err := parseLogRecord(record)
		if err != nil {
			return err
		}
		commit.CheckedOutIn = checkedOut[commit.SHA]
		if !flat {
			commit.Graph = toGraphRow(layout.Next(graph.Commit{SHA: commit.SHA, Parents: parents}))
		}
		appendRow(commit)
		loaded++
		if loaded == 1 || time.Since(lastEmit) >= progressInterval {
			progress(loaded, false)
			lastEmit = time.Now()
		}
		return nil
	})
	errors.As(runErr, &err)
	return loaded, err
}

// writeCommitGraphIfLarge runs `git commit-graph write --reachable` after
// the load's done signal, the first time a load reaches largeRepoRows rows:
// it cuts git log's time-to-first-record and peak memory at real scale.
// Skipped once the file exists. A failure is never surfaced: the file is an
// optimisation, not app state.
func writeCommitGraphIfLarge(wt *session.Worktree, loaded int) {
	if loaded < largeRepoRows {
		return
	}
	graphFile := filepath.Join(wt.Repo.Key, "objects", "info", "commit-graph")
	if _, err := os.Stat(graphFile); err == nil {
		return
	}
	_, _ = wt.Runner.Run(context.Background(), git.Call{
		Dir: wt.Path, Background: true, Timeout: logTimeout,
		Args: []string{"commit-graph", "write", "--reachable"},
	})
}

// refTips is the cheap "did anything move" signature: every
// refs/heads|tags|remotes ref's SHA, plus a synthetic "HEAD" entry —
// "ref:<branch>" if HEAD is on a branch (even unborn), or HEAD's own
// resolved SHA if detached, so a plain branch switch counts as a change.
func refTips(ctx context.Context, r git.Runner, dir string) (map[string]string, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"for-each-ref", "--format=%(refname)%1f%(objectname)%00", "refs/heads", "refs/tags", "refs/remotes"}})
	if err != nil {
		return nil, err
	}
	tips := parseRefTips(result.Stdout)

	branch, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true, Args: []string{"symbolic-ref", "--short", "-q", "HEAD"}})
	detached := quietFailure(err, branch)
	if err != nil && !detached {
		return nil, err
	}
	if !detached {
		tips["HEAD"] = "ref:" + strings.TrimSpace(string(branch.Stdout))
		return tips, nil
	}

	sha, err := resolveHeadSHA(ctx, r, dir) // an unresolvable HEAD reads as ""; the next log load reports it
	if err != nil {
		return nil, err
	}
	tips["HEAD"] = sha
	return tips, nil
}

// parseRefTips parses for-each-ref's "%(refname)%1f%(objectname)%00"
// stream — for-each-ref's own %1f/%00 escape syntax, not git log's
// %x1f/%x00.
func parseRefTips(out []byte) map[string]string {
	tips := map[string]string{}
	for _, record := range nulRecords(out) {
		if name, sha, ok := strings.Cut(record, "\x1f"); ok {
			tips[name] = sha
		}
	}
	return tips
}

func (c *Core) emitLogProgress(repoKey string, loaded int, done bool) {
	payload, _ := json.Marshal(core.LogProgress{Loaded: loaded, Done: done})
	c.sink(core.Event{Name: "log:progress", Repo: repoKey, Payload: payload})
}

// parseLogRecord parses one LogFormat record (7 fields, %x1f-separated) as
// streamed by git log --format=LogFormat.
func parseLogRecord(record []byte) (commit core.LogCommit, parents []string, err error) {
	fields := strings.Split(string(record), "\x1f")
	if len(fields) != 7 {
		return core.LogCommit{}, nil, core.Errorf(core.Internal, "log record has %d fields, want 7", len(fields))
	}
	authorAt, convErr := strconv.ParseInt(fields[4], 10, 64)
	if convErr != nil {
		return core.LogCommit{}, nil, core.Errorf(core.Internal, "log record authorAt: %v", convErr)
	}
	commit = core.LogCommit{
		SHA:      fields[0],
		ShortSHA: fields[1],
		Author:   fields[3],
		AuthorAt: authorAt,
		Refs:     parseRefs(fields[5]),
		Subject:  fields[6],
	}
	return commit, strings.Fields(fields[2]), nil
}

// parseRefs classifies --decorate=full's %D decoration string into labels.
func parseRefs(decoration string) []core.RefLabel {
	if decoration == "" {
		return nil
	}
	var refs []core.RefLabel
	for _, tok := range strings.Split(decoration, ", ") {
		switch {
		case tok == "HEAD":
			refs = append(refs, core.RefLabel{Name: "HEAD", Kind: "head"})
		case strings.HasPrefix(tok, "HEAD -> "):
			refs = append(refs, core.RefLabel{Name: "HEAD", Kind: "head"})
			refs = append(refs, refFromPath(strings.TrimPrefix(tok, "HEAD -> ")))
		case strings.HasPrefix(tok, "tag: "):
			refs = append(refs, refFromPath(strings.TrimPrefix(tok, "tag: ")))
		default:
			refs = append(refs, refFromPath(tok))
		}
	}
	return refs
}

var refKinds = []struct{ prefix, kind string }{
	{"refs/heads/", "branch"}, {"refs/remotes/", "remote"}, {"refs/tags/", "tag"},
}

func refFromPath(path string) core.RefLabel {
	for _, k := range refKinds {
		if name, ok := strings.CutPrefix(path, k.prefix); ok {
			return core.RefLabel{Name: name, Kind: k.kind}
		}
	}
	return core.RefLabel{Name: path}
}

// toGraphRow converts graph's leaf type to core's JSON DTO; the leaf
// package never imports core.
func toGraphRow(row graph.Row) core.GraphRow {
	edges := make([]core.GraphEdge, len(row.Edges))
	for i, e := range row.Edges {
		edges[i] = core.GraphEdge{Lane: e.Lane, Kind: string(e.Kind)}
	}
	return core.GraphRow{Lane: row.Lane, Colour: row.Colour, Edges: edges, Overflow: row.Overflow}
}

// worktreeCheckouts maps a commit SHA to every worktree path currently at
// that HEAD. Any worktree of the repo lists every worktree of it.
func worktreeCheckouts(r git.Runner, dir string, background bool) (map[string][]string, error) {
	result, err := r.Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Background: background,
		Args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if err != nil {
		return nil, err
	}

	checkouts := map[string][]string{}
	for _, wt := range parseWorktreeRecords(result.Stdout) {
		if wt.Path != "" && wt.SHA != "" {
			checkouts[wt.SHA] = append(checkouts[wt.SHA], wt.Path)
		}
	}
	return checkouts, nil
}
