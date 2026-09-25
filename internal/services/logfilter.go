package services

import (
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
// unfiltered load, also what tools/bench times.
func LogArgs(f core.LogFilter) []string {
	return logArgs(f, "", logFormat, "--topo-order", "--parents", "--decorate=full")
}

// searchArgs is SearchLog's message match: a --grep pass over f's
// revisions, author and path, printing only each commit's hash.
func searchArgs(f core.LogFilter, query string) []string {
	return logArgs(f, query, "%H")
}

// logArgs builds a git log call with opts over f, printed in format, and
// matching query against messages if set. Query and Author land only as the
// value half of one '='-joined token (--grep=<q>, --author=<a>), so no
// content can turn them into an option. Branches and Path are separate
// tokens, protected by --end-of-options and -- respectively.
func logArgs(f core.LogFilter, query, format string, opts ...string) []string {
	args := append([]string{"log"}, opts...)
	if query != "" {
		args = append(args, "--grep="+query)
	}
	if f.Author != "" {
		args = append(args, "--author="+f.Author)
	}
	if query != "" || f.Author != "" {
		// plain case-insensitive substring: text containing "(" or "."
		// must not fail or silently mis-match as a regex
		args = append(args, "--regexp-ignore-case", "--fixed-strings")
	}
	if len(f.Branches) == 0 {
		args = append(args, "--exclude=refs/stash", "--all") // same as the unfiltered load
	}
	args = append(args, "--format="+format, "--end-of-options")
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
	filterCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	gen := wt.FilterBegin(filter, cancel)
	go c.streamFilteredLog(filterCtx, wt, filter, gen)
	return nil
}

// streamFilteredLog runs in its own goroutine, mirroring streamLog but
// writing into wt's own filtered view and emitting a Worktree-targeted
// "log:progress" instead of the repo-wide one.
func (c *Core) streamFilteredLog(ctx context.Context, wt *session.Worktree, filter core.LogFilter, gen int) {
	progress := func(loaded int, done bool) { c.emitFilterProgress(wt.ID, gen, loaded, done, nil) }
	loaded, logErr := streamCommits(ctx, wt, filter, false,
		func(commit core.LogCommit) { wt.FilterAppend(gen, commit) }, progress)
	wt.FilterFinish(gen, logErr)
	c.emitFilterProgress(wt.ID, gen, loaded, true, logErr)
}

// emitFilterProgress reports wtID's filtered load. A cancelled one was
// replaced by a newer filter: no failure to show.
func (c *Core) emitFilterProgress(wtID core.WorktreeID, gen, loaded int, done bool, err *core.Error) {
	if err != nil && err.Code == core.Cancelled {
		err = nil
	}
	payload, _ := json.Marshal(core.LogProgress{Loaded: loaded, Done: done, Filtered: true, Generation: gen, Error: err})
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

// SearchLog implements core.API.
func (c *Core) SearchLog(ctx context.Context, id core.WorktreeID, query string) (core.LogSearchResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.LogSearchResult{}, err
	}
	matched, err := matchingSHAs(ctx, wt, query)
	if err != nil {
		return core.LogSearchResult{}, err
	}
	indices, gen, filtered := wt.LogIndicesOf(matched)
	return core.LogSearchResult{Indices: indices, Generation: gen, Filtered: filtered}, nil
}

// matchingSHAs is the set of commits query finds: none for "", the one
// commit a hash resolves to, else every commit in wt's active view whose
// message contains query.
func matchingSHAs(ctx context.Context, wt *session.Worktree, query string) (map[string]bool, error) {
	if query == "" {
		return nil, nil
	}
	if sha, ok := resolveHash(ctx, wt.Runner, wt.Path, query); ok {
		return map[string]bool{sha: true}, nil
	}
	filter, _ := wt.ActiveFilter() // the zero LogFilter (the base log) if none active
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Args: searchArgs(filter, query)})
	if err != nil {
		return nil, core.ErrorOf(err)
	}
	matched := map[string]bool{}
	for sha := range strings.FieldsSeq(string(result.Stdout)) {
		matched[sha] = true
	}
	return matched, nil
}
