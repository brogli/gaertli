package services

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
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

func TestFR_CP_02_OrderedSHAsSortsByCommitDateBothDirections(t *testing.T) {
	byDate := []pickCandidate{
		{SHA: "c30", CommitAt: 30}, {SHA: "c10", CommitAt: 10}, {SHA: "c20", CommitAt: 20},
	}
	got, err := orderedSHAs(t.Context(), git.Runner{}, "", byDate, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"c10", "c20", "c30"}) {
		t.Errorf("oldest-first = %v, want [c10 c20 c30]", got)
	}

	got, err = orderedSHAs(t.Context(), git.Runner{}, "", byDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"c30", "c20", "c10"}) {
		t.Errorf("newest-first = %v, want [c30 c20 c10]", got)
	}
}

func TestFR_CP_02_TiedSHAsFindsCandidatesSharingACommitterDate(t *testing.T) {
	candidates := []pickCandidate{
		{SHA: "a", CommitAt: 10}, {SHA: "b", CommitAt: 20}, {SHA: "c", CommitAt: 20}, {SHA: "d", CommitAt: 30},
	}
	got := tiedSHAs(candidates)
	slices.Sort(got)
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("tiedSHAs = %v, want [b c]", got)
	}

	none := []pickCandidate{{SHA: "a", CommitAt: 1}, {SHA: "b", CommitAt: 2}}
	if got := tiedSHAs(none); len(got) != 0 {
		t.Errorf("tiedSHAs with no shared date = %v, want empty", got)
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
	// without the guard, "git log --no-walk" with no revision defaults to HEAD
	wt := &session.Worktree{}

	err := runSequence(t.Context(), wt, nil, "cherry-pick", "CHERRY_PICK_HEAD", true, core.PickOptions{})

	coretest.AssertCode(t, err, core.Internal)
}

func TestFR_CP_01_CherryPickAppliesACommitOntoCurrentHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{bSHA}, core.PickOptions{}); err != nil {
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

	c, wt := openWorktree(t, repo.Dir)

	err := c.CherryPick(t.Context(), wt, []string{pickSHA}, core.PickOptions{})

	ce := coretest.AsError(t, err, core.GitFailed)
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
	_, _ = (git.Runner{}).Run(t.Context(), git.Call{
		Dir: repo.Dir, Args: []string{"cherry-pick", "--end-of-options", conflSHA},
	})
	stuckHead, err := os.ReadFile(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD"))
	if err != nil {
		t.Fatalf("fixture did not leave CHERRY_PICK_HEAD: %v", err)
	}
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err = c.CherryPick(t.Context(), wt, []string{otherSHA}, core.PickOptions{})
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

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{bSHA}, core.PickOptions{}); err != nil {
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

	c, wt := openWorktree(t, repo.Dir)

	// Deliberately out of order: the service must sort to oldest-first
	// (A, B, C) before calling git, regardless of this call's own order.
	if err := c.CherryPick(t.Context(), wt, []string{cSHA, aSHA, bSHA}, core.PickOptions{}); err != nil {
		t.Fatal(err)
	}

	if got := commitSubjects(t, repo.Dir, "master", "fresh"); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Errorf("commit order on fresh = %v, want [A B C] (oldest first)", got)
	}
	content := readFile(t, repo.Dir, "chain.txt")
	if content != "A\nB\nC\n" {
		t.Errorf("chain.txt = %q, want the cumulative A\\nB\\nC\\n", content)
	}
}

func TestFR_CP_02_CherryPickAppliesParentBeforeChildWhenCommitterDatesTie(t *testing.T) {
	repo := gittest.New(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	gittest.Commit(t, repo.Dir, "seed.txt", "r", "root")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	commitDated(t, repo.Dir, "chain.txt", "A\n", "A", base)
	aSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\n", "B", base)
	bSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\n", "C", base)
	cSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\nD\n", "D", base)
	dSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Run(t, repo.Dir, "branch", "fresh")
	gittest.Run(t, repo.Dir, "checkout", "fresh")

	c, wt := openWorktree(t, repo.Dir)

	// All four share a committer date: only topological order, not the
	// scrambled call order, applies this chain cleanly.
	if err := c.CherryPick(t.Context(), wt, []string{cSHA, aSHA, dSHA, bSHA}, core.PickOptions{}); err != nil {
		t.Fatal(err)
	}

	if got := commitSubjects(t, repo.Dir, "master", "fresh"); !slices.Equal(got, []string{"A", "B", "C", "D"}) {
		t.Errorf("commit order on fresh = %v, want [A B C D] (parent before child)", got)
	}
	content := readFile(t, repo.Dir, "chain.txt")
	if content != "A\nB\nC\nD\n" {
		t.Errorf("chain.txt = %q, want the cumulative A\\nB\\nC\\nD\\n", content)
	}
}

func TestFR_CP_05_CherryPickRefusesTheWholeBatchWhenAMergeCommitIsBuriedInside(t *testing.T) {
	repo := gittest.New(t)
	x1SHA, mSHA, x2SHA := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CherryPick(t.Context(), wt, []string{x1SHA, mSHA, x2SHA}, core.PickOptions{})

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Args["sha"] == "" || ce.Args["verb"] != "cherry-pick" {
		t.Errorf("Args = %+v, want a non-empty sha and verb=cherry-pick", ce.Args)
	}
	if ce.Key != "mergeCommitNotPickable" {
		t.Errorf("Key = %q, want mergeCommitNotPickable", ce.Key)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the pre-check must run before any real cherry-pick)", got, beforeHEAD)
	}
}

func TestFR_RV_03_RevertRefusesTheWholeBatchWhenAMergeCommitIsBuriedInside(t *testing.T) {
	repo := gittest.New(t)
	x1SHA, mSHA, x2SHA := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{x1SHA, mSHA, x2SHA}, core.PickOptions{})

	coretest.AssertCode(t, err, core.GitFailed)
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

	c, wt := openWorktree(t, repo.Dir)

	// Deliberately oldest-first call order: the service must reorder to
	// newest-first (C, then B) before calling git — the reverse order
	// conflicts on this same fixture.
	if err := c.Revert(t.Context(), wt, []string{bSHA, cSHA}, core.PickOptions{}); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, repo.Dir, "chain.txt")
	if content != "A\n" {
		t.Errorf("chain.txt = %q, want exactly A's own line", content)
	}
}

func TestFR_RV_02_RevertAppliesChildBeforeParentWhenCommitterDatesTie(t *testing.T) {
	repo := gittest.New(t)
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	gittest.Commit(t, repo.Dir, "chain.txt", "A\n", "A")
	commitDated(t, repo.Dir, "chain.txt", "A\nB\n", "B", base)
	bSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\n", "C", base)
	cSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\nD\n", "D", base)
	dSHA := gittest.HeadSHA(t, repo.Dir)
	commitDated(t, repo.Dir, "chain.txt", "A\nB\nC\nD\nE\n", "E", base)
	eSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	// B, C, D, E share a committer date: only topological order, not the
	// scrambled call order, reverts this chain cleanly.
	if err := c.Revert(t.Context(), wt, []string{dSHA, bSHA, eSHA, cSHA}, core.PickOptions{}); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, repo.Dir, "chain.txt")
	if content != "A\n" {
		t.Errorf("chain.txt = %q, want back to A's own line", content)
	}
}

func TestFR_RV_01_RevertUndoesACommitLeavingItInHistory(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "adds b.txt")
	bSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Revert(t.Context(), wt, []string{bSHA}, core.PickOptions{}); err != nil {
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

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{revertSHA}, core.PickOptions{})

	ce := coretest.AsError(t, err, core.GitFailed)
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

	c, wt := openWorktree(t, repo.Dir)

	err := c.CherryPick(t.Context(), wt, []string{featureSHA}, core.PickOptions{})

	coretest.AssertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestFR_RV_01_RevertReportsCodeConflictsOnARealStopLeftInPlace(t *testing.T) {
	repo := gittest.New(t)
	c1SHA := realConflictFixture(t, repo.Dir, "revert")

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{c1SHA}, core.PickOptions{})

	coretest.AssertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestFR_CP_06_CherryPickWithRecordOriginAppendsTheCherryPickedFromLine(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{bSHA}, core.PickOptions{RecordOrigin: true}); err != nil {
		t.Fatal(err)
	}

	body := gittest.Out(t, repo.Dir, "log", "-1", "--format=%B")
	want := "(cherry picked from commit " + bSHA + ")"
	if !strings.Contains(body, want) {
		t.Errorf("commit body = %q, want it to contain %q", body, want)
	}
}

func TestFR_CP_06_CherryPickWithNoCommitStagesWithoutAdvancingHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{bSHA}, core.PickOptions{NoCommit: true}); err != nil {
		t.Fatal(err)
	}

	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
	status := statusOf(t, c, wt)
	if len(status.Staged) != 1 || status.Staged[0].Path != "b.txt" {
		t.Errorf("Staged = %+v, want one entry for b.txt", status.Staged)
	}
}

func TestFR_RV_04_RevertWithNoCommitStagesWithoutAdvancingHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "adds b.txt")
	bSHA := gittest.HeadSHA(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Revert(t.Context(), wt, []string{bSHA}, core.PickOptions{NoCommit: true}); err != nil {
		t.Fatal(err)
	}

	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
	status := statusOf(t, c, wt)
	if len(status.Staged) != 1 || status.Staged[0].Path != "b.txt" {
		t.Errorf("Staged = %+v, want one entry for b.txt (the revert's own removal)", status.Staged)
	}
}

// Real git leaves no CHERRY_PICK_HEAD behind for a no-commit cherry-pick
// that stops on a real conflict (checked against git itself, unlike
// -n's own revert equivalent below): the error stays GitFailed rather
// than being remapped to Conflicts, and operation() reports nothing in
// progress, so ConflictBanner never shows. The conflicted file itself
// still surfaces through Status, for the changes view's own resolve flow.
func TestFR_CP_06_CherryPickWithNoCommitOnARealConflictLeavesNoOperationState(t *testing.T) {
	repo := gittest.New(t)
	featureSHA := realConflictFixture(t, repo.Dir, "cherry-pick")

	c, wt := openWorktree(t, repo.Dir)

	err := c.CherryPick(t.Context(), wt, []string{featureSHA}, core.PickOptions{NoCommit: true})

	coretest.AssertCode(t, err, core.GitFailed)
	if fileExists(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD present, want none")
	}
	status, statusErr := c.Status(t.Context(), wt)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if len(status.Conflicted) != 1 || status.Conflicted[0].Path != "conflict.txt" {
		t.Errorf("Conflicted = %+v, want one entry for conflict.txt", status.Conflicted)
	}
}

// Unlike -n cherry-pick, real git does write REVERT_HEAD for a no-commit
// revert that stops on a real conflict, so this one does get remapped to
// Conflicts and ConflictBanner does show, with a working Continue/Abort.
func TestFR_RV_04_RevertWithNoCommitOnARealConflictStillReportsConflicts(t *testing.T) {
	repo := gittest.New(t)
	revertSHA := realConflictFixture(t, repo.Dir, "revert")

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{revertSHA}, core.PickOptions{NoCommit: true})

	coretest.AssertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD missing, want it left in place for the conflict flow")
	}
	status, statusErr := c.Status(t.Context(), wt)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if len(status.Conflicted) != 1 || status.Conflicted[0].Path != "conflict.txt" {
		t.Errorf("Conflicted = %+v, want one entry for conflict.txt", status.Conflicted)
	}
}

// mainlineMergeFixture builds a merge commit with two parents whose own
// trees each differ from the merge's (like mergeCommitFixture), but checks
// out a fresh branch rooted before the divergence, carrying neither
// parent's own file — so cherry-picking the merge against either mainline
// choice applies a real change instead of finding it already present.
func mainlineMergeFixture(t *testing.T, dir string) (mergeSHA string) {
	t.Helper()
	gittest.Commit(t, dir, "base.txt", "base", "base")
	baseSHA := gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "branch", "side")
	gittest.Run(t, dir, "checkout", "side")
	gittest.Commit(t, dir, "side.txt", "side", "side change")
	gittest.Run(t, dir, "checkout", "master")
	gittest.Commit(t, dir, "master2.txt", "master2", "diverging master commit")
	gittest.Run(t, dir, "merge", "--no-ff", "--no-edit", "side")
	mergeSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "checkout", "-b", "fresh", baseSHA)
	return mergeSHA
}

func TestFR_CP_08_CherryPickWithMainlineChoosesTheParentToDiffAgainst(t *testing.T) {
	repo := gittest.New(t)
	mSHA := mainlineMergeFixture(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{mSHA}, core.PickOptions{Mainline: 1}); err != nil {
		t.Fatalf("CherryPick mainline 1 = %v, want nil (no merge-commit refusal)", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "side.txt")); err != nil {
		t.Errorf("side.txt missing after picking against mainline 1: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "master2.txt")); !os.IsNotExist(err) {
		t.Errorf("master2.txt present after picking against mainline 1, want absent (stat err = %v)", err)
	}
}

func TestFR_CP_08_CherryPickWithMainlineTwoDiffersFromMainlineOne(t *testing.T) {
	repo := gittest.New(t)
	mSHA := mainlineMergeFixture(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CherryPick(t.Context(), wt, []string{mSHA}, core.PickOptions{Mainline: 2}); err != nil {
		t.Fatalf("CherryPick mainline 2 = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "master2.txt")); err != nil {
		t.Errorf("master2.txt missing after picking against mainline 2: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "side.txt")); !os.IsNotExist(err) {
		t.Errorf("side.txt present after picking against mainline 2, want absent (stat err = %v) — proves the parent choice is load-bearing", err)
	}
}

func TestFR_RV_04_RevertWithIgnoresMainlineKeepingTheMergeRefusal(t *testing.T) {
	repo := gittest.New(t)
	_, mSHA, _ := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	// Unlike CherryPick, a Mainline choice must not let Revert revert this
	// merge commit (git revert -m 1 would succeed): it refuses exactly like
	// Revert with zero options.
	err := c.Revert(t.Context(), wt, []string{mSHA}, core.PickOptions{Mainline: 1})

	coretest.AssertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the merge refusal must still run before any real revert)", got, beforeHEAD)
	}
}

func TestFR_CP_08_CherryPickWithMainlineOnMultipleCommitsIsAppBugBeforeAnyGitCall(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	aSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	bSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	err := c.CherryPick(t.Context(), wt, []string{aSHA, bSHA}, core.PickOptions{Mainline: 1})

	coretest.AssertCode(t, err, core.Internal)
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("git calls = %d, want 0 (no git call made)", got)
	}
}

// realConflictFixture builds a fixture for verb ("cherry-pick" or "revert")
// across files ("conflict.txt" if none given) where both sides edit the
// same lines, so the call stops with a real content conflict rather than a
// dirty-worktree refusal.
func realConflictFixture(t *testing.T, dir, verb string, files ...string) (sha string) {
	t.Helper()
	if len(files) == 0 {
		files = []string{"conflict.txt"}
	}
	commitAll := func(msg, content string) {
		for _, f := range files {
			writeFile(t, dir, f, content)
		}
		gittest.Run(t, dir, "add", "-A")
		gittest.CommitAll(t, dir, msg)
	}
	if verb == "cherry-pick" {
		commitAll("base", "orig\n")
		gittest.Run(t, dir, "branch", "feature")
		gittest.Run(t, dir, "checkout", "feature")
		commitAll("feature change", "feature\n")
		sha = gittest.HeadSHA(t, dir)
		gittest.Run(t, dir, "checkout", "master")
		commitAll("master change", "master\n")
		return sha
	}
	commitAll("base", "base\n")
	commitAll("applies the change", "applied\n")
	sha = gittest.HeadSHA(t, dir)
	commitAll("further change", "applied2\n")
	return sha
}

// revertConflictFixture builds linear history where HEAD's commit changed
// conflict.txt, with an uncommitted, differing edit to the same file on
// top, so reverting HEAD's commit is refused as "would be overwritten"
// (exit 128).
func revertConflictFixture(t *testing.T, dir string) (revertSHA, beforeHEAD string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, dir, "conflict.txt", "applied\n", "applies the change")
	revertSHA = gittest.HeadSHA(t, dir)
	beforeHEAD = revertSHA
	writeFile(t, dir, "conflict.txt", "dirty")
	return revertSHA, beforeHEAD
}

// cherryPickConflictFixture builds a commit on branch "feature" that
// changes conflict.txt, while "master" has an uncommitted, conflicting edit
// to the same file, so cherry-picking feature's commit is refused as "would
// be overwritten" (exit 128).
func cherryPickConflictFixture(t *testing.T, dir string) (pickSHA, beforeHEAD string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Run(t, dir, "branch", "feature")
	gittest.Run(t, dir, "checkout", "feature")
	gittest.Commit(t, dir, "conflict.txt", "feature\n", "feature change")
	pickSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "checkout", "master")
	beforeHEAD = gittest.HeadSHA(t, dir)
	writeFile(t, dir, "conflict.txt", "dirty")
	return pickSHA, beforeHEAD
}

// commitExists reports whether sha still resolves to a real commit object.
func commitExists(t *testing.T, dir, sha string) bool {
	t.Helper()
	_, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"cat-file", "-e", sha + "^{commit}"},
	})
	return err == nil
}

// mergeCommitFixture builds, on master, a commit x1, a 2-parent merge
// commit m, then a commit x2: a merge buried in a normal run of commits.
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

// commitDated commits like gittest.Commit, but at an explicit date: git's
// timestamps have 1s resolution, so two fast commits could otherwise tie.
func commitDated(t *testing.T, dir, name, content, msg string, at time.Time) {
	t.Helper()
	iso := at.Format(time.RFC3339)
	t.Setenv("GIT_AUTHOR_DATE", iso)
	t.Setenv("GIT_COMMITTER_DATE", iso)
	gittest.Commit(t, dir, name, content, msg)
}

// commitSubjects returns the subjects of the commits reachable from to but
// not from, oldest first.
func commitSubjects(t *testing.T, dir, from, to string) []string {
	t.Helper()
	return strings.Split(gittest.Out(t, dir, "log", "--reverse", "--format=%s", from+".."+to), "\n")
}
