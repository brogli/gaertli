package services

import (
	"context"
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/graph"
	"github.com/brogli/gaertli/internal/session"
)

// largeRepoRows is the loaded-row threshold above which streamLog writes
// git's own commit-graph file once. Tests lower it with Store; atomic
// because writeCommitGraphIfLarge reads it from a streamLog goroutine that
// can still be running after the test that lowered it has returned.
var largeRepoRows atomic.Int64

func init() { largeRepoRows.Store(20_000) }

const progressInterval = 100 * time.Millisecond

// logTimeout bounds the log stream: the runner's 30s default is too short
// for a large repo.
const logTimeout = 10 * time.Minute

// logFormat is the git log record layout LoadLog streams.
const logFormat = "%H%x1f%h%x1f%P%x1f%an%x1f%at%x1f%D%x1f%s%x00"

// LoadLog implements core.API.
func (c *Core) LoadLog(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	gen, ok := wt.Repo.LogBeginLoad()
	if !ok {
		// Another window of this repo loaded it, or is loading it: the
		// caller's window learns the current count the same way.
		view := wt.LogSnapshot(0, 0)
		c.emitLogProgress(wt.Repo.Key, view.Generation, view.Loaded, view.Done, view.Err)
		return nil
	}
	go c.streamLog(context.WithoutCancel(ctx), wt, gen)
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
		Rows: headFor(view.Rows, wt.Path), Loaded: view.Loaded, Done: view.Done, Error: view.Err,
		Generation: view.Generation,
	}, nil
}

// headFor puts the HEAD label on path's own HEAD commit. The log is loaded
// once per repo, so its decoration names the HEAD of whichever worktree
// ran git log.
func headFor(rows []core.LogCommit, path string) []core.LogCommit {
	out := make([]core.LogCommit, len(rows))
	for i, row := range rows {
		refs := slices.DeleteFunc(slices.Clone(row.Refs), func(r core.RefLabel) bool { return r.Kind == core.HeadRef })
		if slices.Contains(row.CheckedOutIn, path) {
			refs = slices.Insert(refs, 0, core.RefLabel{Name: "HEAD", Kind: core.HeadRef})
		}
		row.Refs = refs
		out[i] = row
	}
	return out
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
	if gen, ok := wt.Repo.LogBeginReload(); ok {
		go c.streamLog(context.WithoutCancel(ctx), wt, gen)
	} // else a load is already running; it will record fresh tips itself
	c.reloadFilterIfActive(ctx, wt)
	return nil
}

// reloadFilterIfActive restarts wt's own filtered stream after a ref moved,
// the same way SetLogFilter started it. The filtered view is per-worktree
// (unlike the repo-wide log above), so it needs its own restart regardless
// of which window's base reload wins the race, or whether any of them does.
func (c *Core) reloadFilterIfActive(ctx context.Context, wt *session.Worktree) {
	filterCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	filter, gen, ok := wt.FilterRestart(cancel)
	if !ok {
		cancel() // no filter active: nothing to restart
		return
	}
	go c.streamFilteredLog(filterCtx, wt, filter, gen)
}

// LogIndexOf implements core.API.
func (c *Core) LogIndexOf(ctx context.Context, id core.WorktreeID, sha core.SHA) (core.LogIndexResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.LogIndexResult{}, err
	}
	index, found := wt.Repo.LogIndexOf(sha)
	return core.LogIndexResult{Index: index, Found: found}, nil
}

// streamLog loads the repo's log. It runs detached from the calling RPC,
// which returns at once. gen guards every LogAppend/LogFinish against a
// reload that superseded it.
func (c *Core) streamLog(ctx context.Context, wt *session.Worktree, gen int) {
	repo := wt.Repo
	progress := func(loaded int, done bool) { c.emitLogProgress(repo.Key, gen, loaded, done, nil) }
	loaded, logErr := streamCommits(ctx, wt, core.LogFilter{}, true,
		func(commit core.LogCommit) { repo.LogAppend(gen, commit) }, progress)

	var tips map[string]string
	if logErr == nil {
		// a failure only makes the next ReloadLog reload
		tips, _ = refTips(ctx, wt.Runner, wt.Path)
	}
	repo.LogFinish(gen, logErr, tips)
	c.emitLogProgress(repo.Key, gen, loaded, true, logErr)
	if logErr == nil {
		writeCommitGraphIfLarge(ctx, wt, loaded) // after the done signal: never delays the UI
	}
}

// streamCommits runs filter's git log call, parses every record into a
// LogCommit (with its graph row unless filter is flat), and hands each to
// appendRow; progress fires for the first row and then every
// progressInterval, never with done.
func streamCommits(ctx context.Context, wt *session.Worktree, filter core.LogFilter, background bool,
	appendRow func(core.LogCommit), progress func(loaded int, done bool),
) (loaded int, err *core.Error) {
	checkedOut, ckErr := worktreeCheckouts(ctx, wt.Runner, wt.Path, background)
	if ckErr != nil {
		return 0, core.ErrorOf(ckErr)
	}

	flat := filter.Flat()
	layout := graph.NewLayout()
	var lastEmit time.Time
	_, runErr := wt.Runner.Stream(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true, Background: background, Timeout: logTimeout, Args: LogArgs(filter),
	}, func(record []byte) error {
		commit, parents, err := parseLogRecord(record)
		if err != nil {
			return err
		}
		commit.CheckedOutIn = checkedOut[commit.SHA]
		if !flat {
			commit.Graph = toGraphRow(layout.Next(graph.Commit{
				SHA: string(commit.SHA), Parents: parents, BranchTip: isBranchTip(commit),
			}))
		}
		appendRow(commit)
		loaded++
		if loaded == 1 || time.Since(lastEmit) >= progressInterval {
			progress(loaded, false)
			lastEmit = time.Now()
		}
		return nil
	})
	if runErr != nil {
		return loaded, core.ErrorOf(runErr)
	}
	return loaded, nil
}

func isBranchTip(commit core.LogCommit) bool {
	return slices.ContainsFunc(commit.Refs, func(r core.RefLabel) bool { return r.Kind == core.BranchRef })
}

// writeCommitGraphIfLarge writes git's commit-graph file when a load reaches
// largeRepoRows rows and the repo has none: it cuts git log's
// time-to-first-record and peak memory. A failure is ignored: the file is an
// optimisation, not app state.
func writeCommitGraphIfLarge(ctx context.Context, wt *session.Worktree, loaded int) {
	if int64(loaded) < largeRepoRows.Load() {
		return
	}
	if fileExists(filepath.Join(wt.Repo.Key, "objects", "info", "commit-graph")) {
		return
	}
	_, _ = wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, Background: true, Timeout: logTimeout,
		Args: []string{"commit-graph", "write", "--reachable"},
	})
}

// refTips is the cheap "did anything move" signature: every
// refs/heads|tags|remotes ref's SHA, plus a synthetic "HEAD" entry —
// "ref:<branch>" if HEAD is on a branch (even unborn), or HEAD's own
// resolved SHA if detached, so a plain branch switch counts as a change.
func refTips(ctx context.Context, r git.Runner, dir string) (map[string]string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"for-each-ref", "--format=%(refname)%1f%(objectname)%00", "refs/heads", "refs/tags", "refs/remotes"},
	})
	if err != nil {
		return nil, err
	}
	tips := parseRefTips(result.Stdout)

	branch, detached, err := headBranch(ctx, r, dir)
	if err != nil {
		return nil, err
	}
	if !detached {
		tips["HEAD"] = "ref:" + branch
		return tips, nil
	}
	sha, err := verifyRev(ctx, r, dir, "HEAD")
	if err != nil {
		return nil, err
	}
	tips["HEAD"] = string(sha)
	return tips, nil
}

// parseRefTips parses for-each-ref's "%(refname)%1f%(objectname)%00"
// stream.
func parseRefTips(out []byte) map[string]string {
	tips := map[string]string{}
	for _, record := range nulRecords(out) {
		if name, sha, ok := strings.Cut(record, "\x1f"); ok {
			tips[name] = sha
		}
	}
	return tips
}

func (c *Core) emitLogProgress(repoKey string, gen, loaded int, done bool, err *core.Error) {
	payload, _ := json.Marshal(core.LogProgress{Loaded: loaded, Done: done, Generation: gen, Error: err})
	c.sink(core.Event{Name: "log:progress", Repo: repoKey, Payload: payload})
}

// parseLogRecord parses one logFormat record.
func parseLogRecord(record []byte) (commit core.LogCommit, parents []string, err error) {
	fields := strings.Split(string(record), "\x1f")
	if len(fields) != 7 {
		return core.LogCommit{}, nil, core.Errorf(core.Internal, "log record has %d fields, want 7", len(fields))
	}
	authorAt, convErr := strconv.ParseInt(fields[4], 10, 64)
	if convErr != nil {
		return core.LogCommit{}, nil, core.Errorf(core.Internal, "log record authorAt: %v", convErr)
	}
	parents = strings.Fields(fields[2])
	commit = core.LogCommit{
		SHA:      core.SHA(fields[0]),
		ShortSHA: fields[1],
		Author:   fields[3],
		AuthorAt: authorAt,
		Refs:     parseRefs(fields[5]),
		Subject:  fields[6],
	}
	return commit, parents, nil
}

// parseRefs classifies --decorate=full's %D decoration string into labels.
func parseRefs(decoration string) []core.RefLabel {
	if decoration == "" {
		return nil
	}
	var refs []core.RefLabel
	for tok := range strings.SplitSeq(decoration, ", ") {
		switch {
		case tok == "HEAD":
			refs = append(refs, core.RefLabel{Name: "HEAD", Kind: core.HeadRef})
		case strings.HasPrefix(tok, "HEAD -> "):
			refs = append(refs, core.RefLabel{Name: "HEAD", Kind: core.HeadRef}, refFromPath(strings.TrimPrefix(tok, "HEAD -> ")))
		default:
			refs = append(refs, refFromPath(strings.TrimPrefix(tok, "tag: ")))
		}
	}
	return refs
}

var refKinds = []struct {
	prefix string
	kind   core.RefKind
}{
	{"refs/heads/", core.BranchRef}, {"refs/remotes/", core.RemoteRef}, {"refs/tags/", core.TagRef},
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
		edges[i] = core.GraphEdge{Lane: e.Lane, Kind: core.EdgeKind(e.Kind), Colour: e.Colour}
	}
	return core.GraphRow{Lane: row.Lane, Colour: row.Colour, Edges: edges, Overflow: row.Overflow}
}

// worktreeCheckouts maps a commit SHA to every worktree path currently at
// that HEAD. Any worktree of the repo lists every worktree of it.
func worktreeCheckouts(ctx context.Context, r git.Runner, dir string, background bool) (map[core.SHA][]string, error) {
	worktrees, err := worktreeList(ctx, r, dir, background)
	if err != nil {
		return nil, err
	}
	checkouts := map[core.SHA][]string{}
	for _, wt := range worktrees {
		if wt.Path != "" && wt.SHA != "" {
			checkouts[wt.SHA] = append(checkouts[wt.SHA], wt.Path)
		}
	}
	return checkouts, nil
}
