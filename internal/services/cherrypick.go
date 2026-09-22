package services

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// candidateFormat resolves everything CherryPick/Revert need about each
// selected sha in one process: %ct for FR-CP-02/FR-RV-02's ordering, %P's
// field count for FR-CP-05/FR-RV-03's merge refusal. It is used with
// --no-walk — a plain lookup of exactly these commits, never a walk of the
// history reachable from them, so cousin and unrelated commits work too.
const candidateFormat = "%H%x1f%h%x1f%ct%x1f%P"

// pickCandidate is one selected commit's own resolved facts: ordering
// (CommitAt) and merge-refusal (Merge).
type pickCandidate struct {
	SHA, ShortSHA string
	CommitAt      int64
	Merge         bool
}

// parseCandidates parses candidateFormat records, one per line.
func parseCandidates(out []byte) ([]pickCandidate, error) {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	candidates := make([]pickCandidate, len(lines))
	for i, line := range lines {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 4 {
			return nil, core.Errorf(core.Internal, "candidate record has %d fields, want 4", len(fields))
		}
		at, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, core.Errorf(core.Internal, "candidate commitAt: %v", err)
		}
		candidates[i] = pickCandidate{
			SHA: fields[0], ShortSHA: fields[1], CommitAt: at,
			Merge: len(strings.Fields(fields[3])) >= 2, // 0 (root) or 1 parent: not a merge
		}
	}
	return candidates, nil
}

// firstMerge returns the first merge commit among candidates, if any
// (FR-CP-05/FR-RV-03).
func firstMerge(candidates []pickCandidate) (pickCandidate, bool) {
	for _, c := range candidates {
		if c.Merge {
			return c, true
		}
	}
	return pickCandidate{}, false
}

// orderedSHAs sorts candidates by committer date — ascending for
// cherry-pick (oldest first), descending for revert (newest first).
// Committer date, not author date: it stays monotonic along a branch after
// a rebase. Ties (same second) fall back to the given order, which the
// frontend supplies as log order (newest first): kept for revert, reversed
// for cherry-pick. Git applies the argument order literally, so this is
// the order the commits are applied in.
func orderedSHAs(candidates []pickCandidate, oldestFirst bool) []string {
	sorted := append([]pickCandidate(nil), candidates...)
	if oldestFirst {
		slices.Reverse(sorted)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if oldestFirst {
			return sorted[i].CommitAt < sorted[j].CommitAt
		}
		return sorted[i].CommitAt > sorted[j].CommitAt
	})
	shas := make([]string, len(sorted))
	for i, c := range sorted {
		shas[i] = c.SHA
	}
	return shas
}

// resolveCandidates resolves everything CherryPick/Revert need to know
// about shas in one process. Fails the whole call (nothing attempted) if
// any sha does not resolve: git log --no-walk on a bad object fails
// outright, with no partial output.
func resolveCandidates(ctx context.Context, r git.Runner, dir string, shas []string) ([]pickCandidate, error) {
	args := append([]string{"log", "--no-walk", "--format=" + candidateFormat, "--end-of-options"}, shas...)
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: args})
	if err != nil {
		return nil, err
	}
	return parseCandidates(result.Stdout)
}

// runSequence is CherryPick's and Revert's shared body: resolve shas,
// refuse the whole call up front if any is a merge commit (FR-CP-05/
// FR-RV-03), then run the one real git call under wt's per-worktree write
// lock (ADR-6: cherry-pick/revert write this worktree's index and files).
func runSequence(ctx context.Context, wt *session.Worktree, shas []string, verb, headFile string, oldestFirst bool, gitArgs ...string) error {
	if len(shas) == 0 {
		return core.Errorf(core.Internal, "no commits given")
	}

	wt.Lock()
	defer wt.Unlock()

	candidates, err := resolveCandidates(ctx, wt.Runner, wt.Path, shas)
	if err != nil {
		return err
	}
	if merge, ok := firstMerge(candidates); ok {
		return core.Errorf(core.GitFailed, "%s is a merge commit; %s does not support a merge commit", merge.ShortSHA, verb)
	}

	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: append(gitArgs, orderedSHAs(candidates, oldestFirst)...)})
	return remapConflicts(err, wt.GitDir, headFile)
}

// CherryPick implements core.API.
func (c *Core) CherryPick(ctx context.Context, id core.WorktreeID, shas []string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return runSequence(ctx, wt, shas, "cherry-pick", "CHERRY_PICK_HEAD", true, "cherry-pick", "--end-of-options")
}

// Revert implements core.API. --no-edit: never wait on an interactive
// editor, whether or not this process has a controlling terminal.
func (c *Core) Revert(ctx context.Context, id core.WorktreeID, shas []string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return runSequence(ctx, wt, shas, "revert", "REVERT_HEAD", false, "revert", "--no-edit", "--end-of-options")
}

// remapConflicts classifies a failed cherry-pick/revert: a state file left
// behind under gitDir means the sequencer stopped mid-way, so Code
// Conflicts and the state stays for the conflict flow. No state file means
// git refused before changing anything (a dirty file in the way, another
// operation running): plain GitFailed with git's own message (FR-CP-04).
func remapConflicts(err error, gitDir, headFile string) error {
	if err == nil {
		return nil
	}
	var ce *core.Error
	if errors.As(err, &ce) && ce.Code == core.GitFailed && fileExists(filepath.Join(gitDir, headFile)) {
		ce.Code = core.Conflicts
	}
	return err
}
