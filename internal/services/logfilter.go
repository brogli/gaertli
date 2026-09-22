package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// hashPattern is what resolveHash checks a query against before spending a
// git process on it (spec 22, FR-LOG-05): 4-40 lowercase hex characters —
// git itself accepts a 4-char unambiguous prefix, and lowercase-only avoids
// wasting a rev-parse call on an ordinary text query that happens to look
// hex-ish in the wrong case.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// LogArgs builds the git log call for filter (FR-LOG-05..08); the empty
// filter is the unfiltered load (spec 10), also what tools/bench times. Query
// and Author are user text, but each lands only as the value half of one
// '='-joined argv token (--grep=<q>, --author=<a>) — never its own token —
// so it can never be mistaken for a new option regardless of content
// (TR-10: a flag's value is only ever read as a value). Branches and Path
// are separate tokens carrying user-influenced text, so --end-of-options
// (before the branches) and -- (before the path) protect them exactly like
// spec 20's DiffFile call protects <sha>/<path>.
func LogArgs(f core.LogFilter) []string {
	args := []string{"log", "--topo-order", "--parents", "--decorate=full"}
	if f.Query != "" {
		args = append(args, "--grep="+f.Query)
	}
	if f.Author != "" {
		args = append(args, "--author="+f.Author)
	}
	if f.Query != "" || f.Author != "" {
		// --fixed-strings: plain substring, not a regex (an ordinary query
		// containing "(", ".", etc. must not fail or silently mis-match).
		// --regexp-ignore-case: matches FR-LOG-05's plain "search" framing.
		args = append(args, "--regexp-ignore-case", "--fixed-strings")
	}
	if len(f.Branches) == 0 {
		args = append(args, "--exclude=refs/stash", "--all") // same as the unfiltered load
	}
	args = append(args, "--format="+LogFormat, "--end-of-options")
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
	flat := filter.Query != "" || filter.Author != "" // FR-LOG-02: a text/author search is active
	filterCtx, cancel := context.WithCancel(context.Background())
	gen := wt.FilterBegin(flat, cancel)
	go c.streamFilteredLog(filterCtx, wt, filter, gen, flat)
	return nil
}

// streamFilteredLog runs in its own goroutine, mirroring streamLog (spec 10)
// but writing into wt's own filtered view and emitting a Worktree-targeted
// "log:progress" instead of the repo-wide one spec 10 uses. A query that
// resolves as a commit hash short-circuits every other field (Design: a
// more specific request than any fuzzy filter).
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
// existing commit (Design: a single-record result, not a walk).
func (c *Core) streamFilteredHashHit(wt *session.Worktree, sha string, gen int) {
	commit, err := loadSingleCommit(context.Background(), wt.Runner, wt.Path, sha)
	var logErr *core.Error
	if err == nil {
		if checkedOut, ckErr := worktreeCheckouts(wt.Runner, wt.Path, false); ckErr == nil {
			commit.CheckedOutIn = checkedOut[commit.SHA]
		}
		wt.FilterAppend(gen, commit)
	} else {
		errors.As(err, &logErr)
	}
	wt.FilterFinish(gen, logErr)
	c.emitFilterProgress(wt.ID, 1, true)
}

func (c *Core) emitFilterProgress(wtID core.WorktreeID, loaded int, done bool) {
	payload, _ := json.Marshal(core.LogProgress{Loaded: loaded, Done: done, Filtered: true})
	c.sink(core.Event{Name: "log:progress", Worktree: wtID, Payload: payload})
}

// resolveHash checks whether q looks like, and resolves to, an existing
// commit (git-mapping.md's Search row: "hash via git rev-parse --verify").
// ok is false for anything that doesn't parse as 4-40 lowercase hex
// characters (avoids a wasted git process for every ordinary text query) or
// that parses but does not resolve to a real commit.
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

// loadSingleCommit fetches one LogCommit by sha, reusing LogFormat/
// parseLogRecord (spec 10) via a single-commit git log call instead of a
// second parser. -1 must come before --end-of-options: git's own option
// parser rejects a "-"-prefixed option appearing after --end-of-options as
// a bad revision ("must come before non-option arguments") — confirmed
// empirically; the spec's own draft had the two the other way round.
func loadSingleCommit(ctx context.Context, r git.Runner, dir, sha string) (core.LogCommit, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"log", "--decorate=full", "--format=" + LogFormat, "-1", "--end-of-options", sha},
	})
	if err != nil {
		return core.LogCommit{}, err
	}
	record := bytes.TrimSuffix(result.Stdout, []byte{0, '\n'}) // spec 10's own per-record NUL+newline framing
	commit, _, err := parseLogRecord(record)
	return commit, err
}
