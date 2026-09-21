package services

import (
	"bytes"
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
// git's own commit-graph file once (Design, spec 13, QA-PERF-02, ADR-4
// exception). A package var, not a const, so tests can lower it.
var largeRepoRows = 20_000

const progressInterval = 100 * time.Millisecond

// logTimeout bounds the log stream (reviewed: Runner's 30s default is too
// short for a large repo; the real duration is spec 13's benchmark).
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
	rows, loaded, done, logErr, gen := wt.Repo.LogSnapshot(from, to)
	return core.LogRowsResult{Rows: rows, Loaded: loaded, Done: done, Error: logErr, Generation: gen}, nil
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
		return nil // git-mapping.md: reload only if tips changed
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
	checkedOut, err := worktreeCheckouts(wt.Runner, wt.Path)
	if err != nil {
		var ce *core.Error
		errors.As(err, &ce)
		repo.LogFinish(gen, ce, nil)
		return
	}

	layout := graph.NewLayout()
	loaded, lastEmit := 0, time.Time{}
	_, runErr := wt.Runner.Stream(context.Background(), git.Call{
		Dir: wt.Path, ReadOnly: true, Timeout: logTimeout,
		Args: []string{"log", "--topo-order", "--parents", "--decorate=full",
			"--exclude=refs/stash", "--all", "--format=" + LogFormat},
	}, func(record []byte) error {
		commit, parents, err := parseLogRecord(record)
		if err != nil {
			return err
		}
		commit.CheckedOutIn = checkedOut[commit.SHA]
		commit.Graph = toGraphRow(layout.Next(graph.Commit{SHA: commit.SHA, Parents: parents}))
		repo.LogAppend(gen, commit)
		loaded++
		if loaded == 1 || time.Since(lastEmit) >= progressInterval {
			c.emitLogProgress(repo.Key, loaded, false)
			lastEmit = time.Now()
		}
		return nil
	})

	var logErr *core.Error
	errors.As(runErr, &logErr)

	var tips map[string]string
	if runErr == nil {
		// best-effort: a failure here just means the next ReloadLog always
		// refreshes, rather than failing an otherwise-successful load.
		tips, _ = refTips(context.Background(), wt.Runner, wt.Path)
	}
	repo.LogFinish(gen, logErr, tips)
	c.emitLogProgress(repo.Key, loaded, true)
	if runErr == nil {
		writeCommitGraphIfLarge(wt, loaded) // after the done signal: never delays the UI
	}
}

// writeCommitGraphIfLarge runs `git commit-graph write --reachable` once the
// load's done signal is out, the first time a load reaches largeRepoRows rows: it cuts
// git log's time-to-first-record and peak memory at real scale (spec 13's
// own measurement), at no cost on repos small enough for anyone to notice
// the write. Skipped once the file exists — later opens just find it. Its
// failure is logged (background calls still go through wt.Runner's normal
// command log) but never surfaced: the file is an optimisation, not app
// state (ADR-4's git-own-cache exception).
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
// resolved SHA if detached. A plain branch switch with no new commit moves
// no ref's SHA but does change which branch HEAD names, which is exactly
// what this entry catches.
func refTips(ctx context.Context, r git.Runner, dir string) (map[string]string, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"for-each-ref", "--format=%(refname)%1f%(objectname)%00", "refs/heads", "refs/tags", "refs/remotes"}})
	if err != nil {
		return nil, err
	}
	tips := parseRefTips(result.Stdout)

	branch, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true, Args: []string{"symbolic-ref", "--short", "-q", "HEAD"}})
	var ce *core.Error
	detached := errors.As(err, &ce) && ce.Code == core.GitFailed && branch.ExitCode == 1
	if err != nil && !detached {
		return nil, err
	}
	if !detached {
		tips["HEAD"] = "ref:" + strings.TrimSpace(string(branch.Stdout))
		return tips, nil
	}

	sha, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true, Args: []string{"rev-parse", "-q", "--verify", "HEAD"}})
	if err != nil {
		return nil, err
	}
	tips["HEAD"] = strings.TrimSpace(string(sha.Stdout))
	return tips, nil
}

// parseRefTips parses for-each-ref's "%(refname)%1f%(objectname)%00"
// stream: for-each-ref's own %1f/%00 escape syntax (not %x1f/%x00, unlike
// git log), and the same split-on-NUL-strip-leading-newline record framing
// as git log's own records (log-format-and-decorations.md).
func parseRefTips(out []byte) map[string]string {
	tips := map[string]string{}
	for _, record := range bytes.Split(out, []byte{0}) {
		record = bytes.TrimPrefix(record, []byte("\n"))
		if len(record) == 0 {
			continue
		}
		if name, sha, ok := strings.Cut(string(record), "\x1f"); ok {
			tips[name] = sha
		}
	}
	return tips
}

func (c *Core) emitLogProgress(repoKey string, loaded int, done bool) {
	payload, _ := json.Marshal(struct {
		Loaded int  `json:"loaded"`
		Done   bool `json:"done"`
	}{loaded, done})
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

// parseRefs classifies --decorate=full's %D decoration string into labels
// (Research used: log-format-and-decorations.md).
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

// toGraphRow converts graph's leaf type to core's JSON DTO (spec 05's
// precedent: the leaf package never imports core).
func toGraphRow(row graph.Row) core.GraphRow {
	edges := make([]core.GraphEdge, len(row.Edges))
	for i, e := range row.Edges {
		edges[i] = core.GraphEdge{Lane: e.Lane, Kind: string(e.Kind)}
	}
	return core.GraphRow{Lane: row.Lane, Colour: row.Colour, Edges: edges, Overflow: row.Overflow}
}

// worktreeCheckouts maps a commit SHA to every worktree path currently at
// that HEAD (FR-LOG-10), from "git worktree list --porcelain -z" run in
// dir (any worktree of the repo lists every worktree of it).
func worktreeCheckouts(r git.Runner, dir string) (map[string][]string, error) {
	result, err := r.Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if err != nil {
		return nil, err
	}

	checkouts := map[string][]string{}
	for _, block := range bytes.Split(result.Stdout, []byte{0, 0}) {
		if len(block) == 0 {
			continue
		}
		path, sha := parseWorktreeBlock(block)
		if path != "" && sha != "" {
			checkouts[sha] = append(checkouts[sha], path)
		}
	}
	return checkouts, nil
}

func parseWorktreeBlock(block []byte) (path, sha string) {
	for _, line := range bytes.Split(block, []byte{0}) {
		key, value, _ := strings.Cut(string(line), " ")
		switch key {
		case "worktree":
			path = value
		case "HEAD":
			sha = value
		}
	}
	return path, sha
}
