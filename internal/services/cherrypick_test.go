package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/session"
)

func TestFR_CP_01_ParseCandidatesReadsDateAndMergeFromEachRecord(t *testing.T) {
	records := "root1\x1froot\x1f10\x1f\n" +
		"normal1\x1fnorm\x1f20\x1fparent1\n" +
		"merge1\x1fmerg\x1f30\x1fparent1 parent2\n"

	candidates, err := parseCandidates([]byte(records))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3", len(candidates))
	}
	wantMerge := []bool{false, false, true}
	for i, c := range candidates {
		if c.Merge != wantMerge[i] {
			t.Errorf("candidates[%d].Merge = %v, want %v", i, c.Merge, wantMerge[i])
		}
	}
}

func TestFR_CP_02_OrderedSHAsSortsByCommitDateBothDirectionsWithTieBreak(t *testing.T) {
	byDate := []pickCandidate{
		{SHA: "c30", CommitAt: 30}, {SHA: "c10", CommitAt: 10}, {SHA: "c20", CommitAt: 20},
	}
	if got := orderedSHAs(byDate, true); !equalSHAs(got, []string{"c10", "c20", "c30"}) {
		t.Errorf("oldest-first = %v, want [c10 c20 c30]", got)
	}
	if got := orderedSHAs(byDate, false); !equalSHAs(got, []string{"c30", "c20", "c10"}) {
		t.Errorf("newest-first = %v, want [c30 c20 c10]", got)
	}

	tied := []pickCandidate{
		{SHA: "n3", CommitAt: 5}, {SHA: "n2", CommitAt: 5}, {SHA: "n1", CommitAt: 5},
	}
	if got := orderedSHAs(tied, true); !equalSHAs(got, []string{"n1", "n2", "n3"}) {
		t.Errorf("tied oldest-first = %v, want [n1 n2 n3] (given order reversed)", got)
	}
	if got := orderedSHAs(tied, false); !equalSHAs(got, []string{"n3", "n2", "n1"}) {
		t.Errorf("tied newest-first = %v, want [n3 n2 n1] (given order kept)", got)
	}
}

func TestFR_CP_05_FirstMergeFindsTheMergeCandidateOrReportsNone(t *testing.T) {
	normal1 := pickCandidate{SHA: "a"}
	merge := pickCandidate{SHA: "b", Merge: true}
	normal2 := pickCandidate{SHA: "c"}

	got, ok := firstMerge([]pickCandidate{normal1, merge, normal2})
	if !ok || got.SHA != "b" {
		t.Errorf("firstMerge = %v, %v, want b, true", got, ok)
	}

	_, ok = firstMerge([]pickCandidate{normal1, normal2})
	if ok {
		t.Error("firstMerge with no merge = ok true, want false")
	}
}

func TestFR_CP_01_RunSequenceRejectsEmptyShasBeforeAnyGitCall(t *testing.T) {
	// A bare session.Worktree{} (no gittest repo): real git is on PATH in
	// every test, and a bare "git log --no-walk --end-of-options" with no
	// trailing revision silently defaults to HEAD, so this
	// guard needs a real regression test, not a tautology.
	wt := &session.Worktree{}

	err := runSequence(context.Background(), wt, nil, "cherry-pick", "CHERRY_PICK_HEAD", true, "cherry-pick", "--end-of-options")

	_ = assertCode(t, err, core.Internal)
}

func TestFR_CP_01_CherryPickAppliesACommitOntoCurrentHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.CherryPick(context.Background(), wt.ID, []string{bSHA}); err != nil {
		t.Fatal(err)
	}

	if got := currentBranch(t, repo.Dir); got != "master" {
		t.Errorf("current branch = %s, want master", got)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "b.txt")); err != nil {
		t.Errorf("b.txt missing after cherry-pick: %v", err)
	}
	if got := branchTip(t, repo.Dir, "feature"); got != bSHA {
		t.Errorf("feature = %s, want unchanged %s", got, bSHA)
	}
}

func TestFR_CP_04_CherryPickRefusesWithADirtyFileInTheWay(t *testing.T) {
	repo := gittest.New(t)
	pickSHA, beforeHEAD := cherryPickConflictFixture(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.CherryPick(context.Background(), wt.ID, []string{pickSHA})

	ce := assertCode(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "would be overwritten") {
		t.Errorf("GitStderr = %q, want it to say the file would be overwritten", ce.GitStderr)
	}
	if fileExists(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD present, want none")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_CP_04_CherryPickFailsWhenAnotherPickIsAlreadyStuck(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "conflict.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "branch", "confl")
	gittest.Run(t, repo.Dir, "checkout", "confl")
	gittest.Commit(t, repo.Dir, "conflict.txt", "confl\n", "confl change")
	conflSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master\n", "master change")

	// The other, unrelated sha the second call tries: created now, while
	// the tree is still clean — switching branches once a pick is stuck
	// below is refused by git itself (unmerged paths), so this must not
	// happen after that point.
	gittest.Run(t, repo.Dir, "branch", "other")
	gittest.Run(t, repo.Dir, "checkout", "other")
	gittest.Commit(t, repo.Dir, "other.txt", "x", "unrelated commit")
	otherSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	// Real conflict, set up directly via the runner, not via our own
	// CherryPick — leaves CHERRY_PICK_HEAD stuck, unresolved.
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{
		Dir: repo.Dir, Args: []string{"cherry-pick", "--end-of-options", conflSHA},
	})
	stuckHead, err := os.ReadFile(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD"))
	if err != nil {
		t.Fatalf("fixture did not leave CHERRY_PICK_HEAD: %v", err)
	}
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, openErr := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if openErr != nil {
		t.Fatal(openErr)
	}

	err = c.CherryPick(context.Background(), wt.ID, []string{otherSHA})
	if err == nil {
		t.Fatal("CherryPick = nil, want an error while a prior pick is stuck")
	}

	gotHead, readErr := os.ReadFile(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD"))
	if readErr != nil {
		t.Fatalf("CHERRY_PICK_HEAD missing after the second call: %v", readErr)
	}
	if string(gotHead) != string(stuckHead) {
		t.Errorf("CHERRY_PICK_HEAD = %q, want unchanged %q", gotHead, stuckHead)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
	if _, statErr := os.Stat(filepath.Join(repo.Dir, "other.txt")); statErr == nil {
		t.Error("other.txt applied, want the second cherry-pick's changes not applied")
	}
}

func TestFR_CP_01_CherryPickWorksOnADetachedHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	baseSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "--detach", baseSHA)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.CherryPick(context.Background(), wt.ID, []string{bSHA}); err != nil {
		t.Fatal(err)
	}

	if got := currentBranch(t, repo.Dir); got != "" {
		t.Errorf("current branch = %s, want still detached", got)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got == baseSHA {
		t.Error("HEAD did not advance")
	}
}

func TestFR_CP_02_CherryPickAppliesOldestFirstRegardlessOfCallOrder(t *testing.T) {
	repo := gittest.New(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	gittest.Commit(t, repo.Dir, "seed.txt", "r", "root")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	commitDated(t, repo.Dir, "chain.txt", "A\n", "A", base)
	aSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\n", "B", base.Add(time.Minute))
	bSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\n", "C", base.Add(2*time.Minute))
	cSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Run(t, repo.Dir, "branch", "fresh")
	gittest.Run(t, repo.Dir, "checkout", "fresh")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	// Deliberately out of order: the service must sort to oldest-first
	// (A, B, C) before calling git, regardless of this call's own order.
	if err := c.CherryPick(context.Background(), wt.ID, []string{cSHA, aSHA, bSHA}); err != nil {
		t.Fatal(err)
	}

	if got := commitSubjects(t, repo.Dir, "master", "fresh"); !equalSHAs(got, []string{"A", "B", "C"}) {
		t.Errorf("commit order on fresh = %v, want [A B C] (oldest first)", got)
	}
	content, readErr := os.ReadFile(filepath.Join(repo.Dir, "chain.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "A\nB\nC\n" {
		t.Errorf("chain.txt = %q, want the cumulative A\\nB\\nC\\n", content)
	}
}

func TestFR_CP_05_CherryPickRefusesTheWholeBatchWhenAMergeCommitIsBuriedInside(t *testing.T) {
	repo := gittest.New(t)
	x1SHA, mSHA, x2SHA := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.CherryPick(context.Background(), wt.ID, []string{x1SHA, mSHA, x2SHA})

	_ = assertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the pre-check must run before any real cherry-pick)", got, beforeHEAD)
	}
}

func TestFR_RV_03_RevertRefusesTheWholeBatchWhenAMergeCommitIsBuriedInside(t *testing.T) {
	repo := gittest.New(t)
	x1SHA, mSHA, x2SHA := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Revert(context.Background(), wt.ID, []string{x1SHA, mSHA, x2SHA})

	_ = assertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the pre-check must run before any real revert)", got, beforeHEAD)
	}
}

func TestFR_RV_02_RevertAppliesNewestFirstRegardlessOfCallOrder(t *testing.T) {
	repo := gittest.New(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	commitDated(t, repo.Dir, "chain.txt", "A\n", "A", base)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\n", "B", base.Add(time.Minute))
	bSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\n", "C", base.Add(2*time.Minute))
	cSHA := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	// Deliberately oldest-first call order: the service must reorder to
	// newest-first (C, then B) before calling git — the reverse order
	// conflicts on this same fixture.
	if err := c.Revert(context.Background(), wt.ID, []string{bSHA, cSHA}); err != nil {
		t.Fatal(err)
	}

	content, readErr := os.ReadFile(filepath.Join(repo.Dir, "chain.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "A\n" {
		t.Errorf("chain.txt = %q, want exactly A's own line", content)
	}
}

func TestFR_RV_01_RevertUndoesACommitLeavingItInHistory(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "adds b.txt")
	bSHA := gittest.HeadSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Revert(context.Background(), wt.ID, []string{bSHA}); err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(filepath.Join(repo.Dir, "b.txt")); !os.IsNotExist(statErr) {
		t.Errorf("b.txt still present after revert (stat err = %v)", statErr)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got == bSHA {
		t.Error("HEAD did not advance with a new revert commit")
	}
	if !commitExists(t, repo.Dir, bSHA) {
		t.Error("B's own commit is gone from history, want it untouched")
	}
}

func TestFR_RV_03_RevertRefusesWithADirtyFileInTheWay(t *testing.T) {
	repo := gittest.New(t)
	revertSHA, beforeHEAD := revertConflictFixture(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Revert(context.Background(), wt.ID, []string{revertSHA})

	ce := assertCode(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "would be overwritten") {
		t.Errorf("GitStderr = %q, want it to say the file would be overwritten", ce.GitStderr)
	}
	if fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD present, want none")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_CP_01_CherryPickReportsCodeConflictsOnARealStopLeftInPlace(t *testing.T) {
	repo := gittest.New(t)
	featureSHA := realConflictFixture(t, repo.Dir, "cherry-pick")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.CherryPick(context.Background(), wt.ID, []string{featureSHA})

	_ = assertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestFR_RV_01_RevertReportsCodeConflictsOnARealStopLeftInPlace(t *testing.T) {
	repo := gittest.New(t)
	c1SHA := realConflictFixture(t, repo.Dir, "revert")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Revert(context.Background(), wt.ID, []string{c1SHA})

	_ = assertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD missing, want it left in place for the conflict flow")
	}
}

// realConflictFixture builds case 15's own fixture for verb ("cherry-pick"
// or "revert"): both sides genuinely edit the same line, so the call stops
// with a real content conflict rather than the
// dirty-worktree-class refusal cases 7/14 exercise.
func realConflictFixture(t *testing.T, dir, verb string) (sha string) {
	t.Helper()
	if verb == "cherry-pick" {
		gittest.Commit(t, dir, "conflict.txt", "orig\n", "base")
		gittest.Run(t, dir, "branch", "feature")
		gittest.Run(t, dir, "checkout", "feature")
		gittest.Commit(t, dir, "conflict.txt", "feature\n", "feature change")
		sha = gittest.HeadSHA(t, dir)
		gittest.Run(t, dir, "checkout", "master")
		gittest.Commit(t, dir, "conflict.txt", "master\n", "master change")
		return sha
	}
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, dir, "conflict.txt", "applied\n", "applies the change")
	sha = gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "conflict.txt", "applied2\n", "further change")
	return sha
}

// revertConflictFixture builds case 14's own fixture: linear history where
// HEAD's own commit changed conflict.txt, and an uncommitted, differing
// edit to the same file sits on top — so reverting HEAD's commit is
// refused as "would be overwritten" (exit 128). Cherry-pick's two-branch
// shape would make this a no-op revert: the target branch never carried the
// change being reverted.
func revertConflictFixture(t *testing.T, dir string) (revertSHA, beforeHEAD string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, dir, "conflict.txt", "applied\n", "applies the change")
	revertSHA = gittest.HeadSHA(t, dir)
	beforeHEAD = revertSHA
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	return revertSHA, beforeHEAD
}

// cherryPickConflictFixture builds case 7's own fixture: a commit on
// branch "feature" that changes conflict.txt, while the current branch
// "master" has an uncommitted, conflicting edit to the same file — so
// cherry-picking feature's own commit is refused as "would be overwritten"
// (exit 128).
func cherryPickConflictFixture(t *testing.T, dir string) (pickSHA, beforeHEAD string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Run(t, dir, "branch", "feature")
	gittest.Run(t, dir, "checkout", "feature")
	gittest.Commit(t, dir, "conflict.txt", "feature\n", "feature change")
	pickSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "checkout", "master")
	beforeHEAD = gittest.HeadSHA(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	return pickSHA, beforeHEAD
}

// commitExists reports whether sha still resolves to a real commit object.
func commitExists(t *testing.T, dir, sha string) bool {
	t.Helper()
	_, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"cat-file", "-e", sha + "^{commit}"},
	})
	return err == nil
}

// mergeCommitFixture builds cases 9/13's own fixture on master: a normal
// commit x1, a real 2-parent merge commit m (merging a side branch back
// in), then another normal commit x2 — a merge commit buried inside an
// otherwise normal run of commits.
func mergeCommitFixture(t *testing.T, dir string) (x1SHA, mSHA, x2SHA string) {
	t.Helper()
	gittest.Commit(t, dir, "base.txt", "base", "base")
	gittest.Commit(t, dir, "x1.txt", "x1", "x1")
	x1SHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "branch", "side")
	gittest.Run(t, dir, "checkout", "side")
	gittest.Commit(t, dir, "side.txt", "side", "side change")
	gittest.Run(t, dir, "checkout", "master")
	gittest.Commit(t, dir, "master2.txt", "master2", "diverging master commit")
	gittest.Run(t, dir, "merge", "--no-ff", "--no-edit", "side")
	mSHA = gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "x2.txt", "x2", "x2")
	x2SHA = gittest.HeadSHA(t, dir)
	return x1SHA, mSHA, x2SHA
}

// commitDated commits like commit, but at an explicit, well-separated
// author/committer date: git's own commit timestamp has 1s resolution, so
// a fast test run can otherwise land two commits in the same second,
// defeating an ordering test's own point (proving the date-based sort, not
// its given-order tie-break).
func commitDated(t *testing.T, dir, name, content, msg string, at time.Time) {
	t.Helper()
	iso := at.Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", iso)
	t.Setenv("GIT_COMMITTER_DATE", iso)
	gittest.Commit(t, dir, name, content, msg)
}

// commitSubjects returns the subjects of the commits reachable from to but
// not from, oldest first — the commits a cherry-pick/revert call itself
// added on top of a shared base.
func commitSubjects(t *testing.T, dir, from, to string) []string {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"log", "--reverse", "--format=%s", from + ".." + to},
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
}

func equalSHAs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
