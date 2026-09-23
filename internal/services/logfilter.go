package services

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// hashPattern is what resolveHash checks a query against before spending a
// git process on it: git accepts a 4-char unambiguous prefix, and
// lowercase-only keeps ordinary text queries from costing a rev-parse.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// LogArgs builds the git log call for filter; the empty filter is the
// unfiltered load, also what tools/bench times. Query and Author land only
// as the value half of one '='-joined token (--grep=<q>, --author=<a>), so
// no content can turn them into an option. Branches and Path are separate
// tokens, protected by --end-of-options and -- respectively.
func LogArgs(f core.LogFilter) []string {
	args := []string{"log", "--topo-order", "--parents", "--decorate=full"}
	if f.Query != "" {
		args = append(args, "--grep="+f.Query)
	}
	if f.Author != "" {
		args = append(args, "--author="+f.Author)
	}
	if f.Query != "" || f.Author != "" {
		// plain case-insensitive substring: a query containing "(" or "."
		// must not fail or silently mis-match as a regex
		args = append(args, "--regexp-ignore-case", "--fixed-strings")
	}
	if len(f.Branches) == 0 {
		args = append(args, "--exclude=refs/stash", "--all") // same as the unfiltered load
	}
	args = append(args, "--format="+logFormat, "--end-of-options")
	args = append(args, f.Branches...)
	if f.Path != "" {
		args = append(args, "--", f.Path)
	}
	return args
}

// SetLogFilter implements core.API.
func (c *Core) SetLogFilter(ctx context.Context, id core.WorktreeID, filter core.LogFilter) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if filter.Empty() {
		wt.FilterClear()
		return nil
	}
	flat := filter.Query != "" || filter.Author != "" // a search result has no graph
	filterCtx, cancel := context.WithCancel(context.Background())
	gen := wt.FilterBegin(flat, cancel)
	go c.streamFilteredLog(filterCtx, wt, filter, gen, flat)
	return nil
}

// streamFilteredLog runs in its own goroutine, mirroring streamLog but
// writing into wt's own filtered view and emitting a Worktree-targeted
// "log:progress" instead of the repo-wide one. A query that resolves as a
// commit hash short-circuits every other field: a pasted hash is a more
// specific request than any fuzzy filter.
func (c *Core) streamFilteredLog(ctx context.Context, wt *session.Worktree, filter core.LogFilter, gen int, flat bool) {
	if sha, ok := resolveHash(ctx, wt.Runner, wt.Path, filter.Query); ok {
		c.streamFilteredHashHit(wt, sha, gen)
		return
	}
	progress := func(loaded int, done bool) { c.emitFilterProgress(wt.ID, loaded, done) }
	loaded, logErr := streamCommits(ctx, wt, LogArgs(filter), flat, false,
		func(commit core.LogCommit) { wt.FilterAppend(gen, commit) }, progress)
	wt.FilterFinish(gen, logErr)
	progress(loaded, true)
}

// streamFilteredHashHit finishes a filtered load whose Query resolved to an
// existing commit: a single-record result, not a walk.
func (c *Core) streamFilteredHashHit(wt *session.Worktree, sha string, gen int) {
	var logErr *core.Error
	if commit, err := loadSingleCommit(context.Background(), wt.Runner, wt.Path, sha); err != nil {
		logErr = core.ErrorOf(err)
	} else {
		if checkedOut, ckErr := worktreeCheckouts(wt.Runner, wt.Path, false); ckErr == nil {
			commit.CheckedOutIn = checkedOut[commit.SHA]
		}
		wt.FilterAppend(gen, commit)
	}
	wt.FilterFinish(gen, logErr)
	c.emitFilterProgress(wt.ID, 1, true)
}

func (c *Core) emitFilterProgress(wtID core.WorktreeID, loaded int, done bool) {
	payload, _ := json.Marshal(core.LogProgress{Loaded: loaded, Done: done, Filtered: true})
	c.sink(core.Event{Name: "log:progress", Worktree: wtID, Payload: payload})
}

// resolveHash checks whether q looks like, and resolves to, an existing
// commit.
func resolveHash(ctx context.Context, r git.Runner, dir, q string) (sha string, ok bool) {
	if !hashPattern.MatchString(q) {
		return "", false
	}
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"rev-parse", "--quiet", "--verify", "--end-of-options", q + "^{commit}"},
	})
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(result.Stdout)), true
}

// loadSingleCommit fetches one LogCommit by sha, reusing logFormat and
// parseLogRecord instead of a second parser. -1 must come before
// --end-of-options: git rejects a "-"-prefixed option after it as a bad
// revision.
func loadSingleCommit(ctx context.Context, r git.Runner, dir, sha string) (core.LogCommit, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"log", "--decorate=full", "--format=" + logFormat, "-1", "--end-of-options", sha},
	})
	if err != nil {
		return core.LogCommit{}, err
	}
	record := bytes.TrimSuffix(result.Stdout, []byte{0, '\n'}) // per-record NUL+newline framing
	commit, _, err := parseLogRecord(record)
	return commit, err
}
