package services

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// candidateFormat resolves everything CherryPick/Revert need about each
// selected sha in one process: %ct for the apply order, %P's field count
// for the merge refusal. It is used with --no-walk, a lookup of exactly
// these commits, so cousin and unrelated commits work too.
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

// firstMerge returns the first merge commit among candidates, if any.
func firstMerge(candidates []pickCandidate) (pickCandidate, bool) {
	for _, c := range candidates {
		if c.Merge {
			return c, true
		}
	}
	return pickCandidate{}, false
}

// tiedSHAs returns the shas of candidates that share their committer date
// with at least one other candidate.
func tiedSHAs(candidates []pickCandidate) []string {
	counts := make(map[int64]int, len(candidates))
	for _, c := range candidates {
		counts[c.CommitAt]++
	}
	var tied []string
	for _, c := range candidates {
		if counts[c.CommitAt] > 1 {
			tied = append(tied, c.SHA)
		}
	}
	return tied
}

// topoOrder resolves shas' topological order, children before parents, as
// "git rev-list --topo-order" reports them.
func topoOrder(ctx context.Context, r git.Runner, dir string, shas []string) ([]string, error) {
	args := append([]string{"rev-list", "--topo-order", "--end-of-options"}, shas...)
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: args})
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(result.Stdout), "\n"), "\n"), nil
}

// orderedSHAs sorts candidates by committer date — ascending for
// cherry-pick (oldest first), descending for revert (newest first).
// Committer date, not author date: it stays monotonic along a branch after
// a rebase. Ties (same second) are broken by topological order, children
// before parents, reversed along with the rest for cherry-pick: the
// selection order the frontend passes carries no ordering guarantee of its
// own. Git applies the argument order literally, so this is the order the
// commits are applied in.
func orderedSHAs(ctx context.Context, r git.Runner, dir string, candidates []pickCandidate, oldestFirst bool) ([]string, error) {
	rank := map[string]int{}
	if tied := tiedSHAs(candidates); len(tied) > 0 {
		order, err := topoOrder(ctx, r, dir, tied)
		if err != nil {
			return nil, err
		}
		for i, sha := range order {
			rank[sha] = i
		}
	}

	sorted := slices.Clone(candidates)
	slices.SortStableFunc(sorted, func(a, b pickCandidate) int {
		if a.CommitAt != b.CommitAt {
			return cmp.Compare(b.CommitAt, a.CommitAt) // newest first
		}
		return cmp.Compare(rank[a.SHA], rank[b.SHA]) // children before parents
	})
	if oldestFirst {
		slices.Reverse(sorted)
	}

	shas := make([]string, len(sorted))
	for i, c := range sorted {
		shas[i] = c.SHA
	}
	return shas, nil
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

// pickArgs builds verb's flags from opts. revert always gets --no-edit, so
// it never waits on an editor, and never -x.
func pickArgs(verb string, opts core.PickOptions) []string {
	args := []string{verb}
	if opts.NoCommit {
		args = append(args, "-n")
	}
	if opts.RecordOrigin && verb == "cherry-pick" {
		args = append(args, "-x")
	}
	if verb == "revert" {
		args = append(args, "--no-edit")
	}
	return append(args, "--end-of-options")
}

// runSequence is CherryPick's and Revert's shared body: resolve
// shas, refuse the whole call up front if any is a merge commit, then run
// verb under wt's write lock. A failure that leaves headFile behind stopped
// mid-way: Code Conflicts.
func runSequence(ctx context.Context, wt *session.Worktree, shas []string, verb, headFile string, oldestFirst bool, opts core.PickOptions) error {
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
		args := map[string]string{"sha": merge.ShortSHA, "verb": verb}
		return core.ErrorfArgs(core.GitFailed, "mergeCommitNotPickable", args, "%s is a merge commit; %s does not support a merge commit", merge.ShortSHA, verb)
	}

	shas, err = orderedSHAs(ctx, wt.Runner, wt.Path, candidates, oldestFirst)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: append(pickArgs(verb, opts), shas...)})
	return remapIfPending(err, fileExists(filepath.Join(wt.GitDir, headFile)))
}

// CherryPick implements core.API.
func (c *Core) CherryPick(ctx context.Context, id core.WorktreeID, shas []string, opts core.PickOptions) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return runSequence(ctx, wt, shas, "cherry-pick", "CHERRY_PICK_HEAD", true, opts)
}

// Revert implements core.API.
func (c *Core) Revert(ctx context.Context, id core.WorktreeID, shas []string, opts core.PickOptions) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	return runSequence(ctx, wt, shas, "revert", "REVERT_HEAD", false, opts)
}
