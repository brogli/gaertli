// Requirements: FR-CP-01, FR-CP-02, FR-CP-04, FR-CP-05, FR-CP-06, FR-RV-01, FR-RV-02, FR-RV-03, FR-RV-04

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

func TestParseCandidatesReadsDateAndMergeFromEachRecord(t *testing.T) {
	t.Parallel()
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

func TestOrderedSHAsSortsByCommitDateBothDirections(t *testing.T) {
	t.Parallel()
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

func TestTiedSHAsFindsCandidatesSharingACommitterDate(t *testing.T) {
	t.Parallel()
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

func TestFirstMergeFindsTheMergeCandidateOrReportsNone(t *testing.T) {
	t.Parallel()
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

func TestRunSequenceRejectsEmptyShasBeforeAnyGitCall(t *testing.T) {
	t.Parallel()
	// without the guard, "git log --no-walk" with no revision defaults to HEAD
	wt := &session.Worktree{}

	err := runSequence(t.Context(), wt, nil, "cherry-pick", "CHERRY_PICK_HEAD", true, core.PickOptions{})

	coretest.AssertCode(t, err, core.Internal)
}

func TestCherryPickAppliesACommitOntoCurrentHEAD(t *testing.T) {
	t.Parallel()
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

func TestCherryPickRefusesWithADirtyFileInTheWay(t *testing.T) {
	t.Parallel()
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

func TestCherryPickFailsWhenAnotherPickIsAlreadyStuck(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	shas := gittest.History(t, repo.Dir,
		gittest.Step{Message: "base", Files: map[string]string{"conflict.txt": "base\n"}},
		gittest.Step{Branch: "confl", From: "master", Message: "confl change", Files: map[string]string{"conflict.txt": "confl\n"}},
		gittest.Step{Branch: "confl", Message: "unrelated commit", Files: map[string]string{"other.txt": "x"}},
		gittest.Step{Message: "master change", Files: map[string]string{"conflict.txt": "master\n"}},
	)
	conflSHA, otherSHA := shas[1], shas[2]

	// A conflict made with git directly, not CherryPick: leaves
	// CHERRY_PICK_HEAD.
	tryGit(t, repo.Dir, "cherry-pick", "--end-of-options", conflSHA)
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

func TestCherryPickWorksOnADetachedHEAD(t *testing.T) {
	t.Parallel()
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

func TestCherryPickAppliesParentBeforeChildWhenCommitterDatesTie(t *testing.T) {
	t.Parallel()
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

	if got := gittest.Out(t, repo.Dir, "log", "--reverse", "--format=%s", "master..fresh"); got != "A\nB\nC\nD" {
		t.Errorf("commit order on fresh = %q, want A, B, C, D (parent before child)", got)
	}
	content := readFile(t, repo.Dir, "chain.txt")
	if content != "A\nB\nC\nD\n" {
		t.Errorf("chain.txt = %q, want the cumulative A\\nB\\nC\\nD\\n", content)
	}
}

func TestCherryPickAndRevertRefuseTheWholeBatchWhenAMergeCommitIsBuriedInside(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	x1SHA, mSHA, x2SHA := mergeCommitFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)
	c, wt := openWorktree(t, repo.Dir)
	batch := []string{x1SHA, mSHA, x2SHA}

	err := c.CherryPick(t.Context(), wt, batch, core.PickOptions{})

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Args["sha"] == "" || ce.Args["verb"] != "cherry-pick" {
		t.Errorf("Args = %+v, want a non-empty sha and verb=cherry-pick", ce.Args)
	}
	if ce.Key != "mergeCommitNotPickable" {
		t.Errorf("Key = %q, want mergeCommitNotPickable", ce.Key)
	}

	coretest.AssertCode(t, c.Revert(t.Context(), wt, batch, core.PickOptions{}), core.GitFailed)

	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the pre-check must run before any real pick or revert)", got, beforeHEAD)
	}
	for _, state := range []string{"CHERRY_PICK_HEAD", "REVERT_HEAD", "sequencer"} {
		if fileExists(filepath.Join(repo.Dir, ".git", state)) {
			t.Errorf(".git/%s present, want no operation in progress", state)
		}
	}
}

func TestRevertAppliesChildBeforeParentWhenCommitterDatesTie(t *testing.T) {
	t.Parallel()
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

func TestRevertUndoesACommitLeavingItInHistory(t *testing.T) {
	t.Parallel()
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

func TestRevertRefusesWithADirtyFileInTheWay(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	revertSHA := revertConflictFixture(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{revertSHA}, core.PickOptions{})

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "would be overwritten") {
		t.Errorf("GitStderr = %q, want it to say the file would be overwritten", ce.GitStderr)
	}
	if fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD present, want none")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != revertSHA {
		t.Errorf("HEAD = %s, want unchanged %s", got, revertSHA)
	}
}

func TestCherryPickReportsCodeConflictsOnARealStopLeftInPlace(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	featureSHA := realConflictFixture(t, repo.Dir, "cherry-pick")

	c, wt := openWorktree(t, repo.Dir)

	err := c.CherryPick(t.Context(), wt, []string{featureSHA}, core.PickOptions{})

	coretest.AssertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestRevertReportsCodeConflictsOnARealStopLeftInPlace(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	c1SHA := realConflictFixture(t, repo.Dir, "revert")

	c, wt := openWorktree(t, repo.Dir)

	err := c.Revert(t.Context(), wt, []string{c1SHA}, core.PickOptions{})

	coretest.AssertCode(t, err, core.Conflicts)
	if !fileExists(filepath.Join(repo.Dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD missing, want it left in place for the conflict flow")
	}
}

func TestCherryPickWithRecordOriginAppendsTheCherryPickedFromLine(t *testing.T) {
	t.Parallel()
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

func TestCherryPickWithNoCommitStagesWithoutAdvancingHEAD(t *testing.T) {
	t.Parallel()
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

func TestRevertWithNoCommitStagesWithoutAdvancingHEAD(t *testing.T) {
	t.Parallel()
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

// git leaves no CHERRY_PICK_HEAD for a no-commit cherry-pick stopped on a
// conflict: the error stays GitFailed and no operation is in progress. The
// conflicted file still shows in Status.
func TestCherryPickWithNoCommitOnARealConflictLeavesNoOperationState(t *testing.T) {
	t.Parallel()
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

// Unlike a no-commit cherry-pick, a no-commit revert stopped on a conflict
// leaves REVERT_HEAD, so the error is Conflicts.
func TestRevertWithNoCommitOnARealConflictStillReportsConflicts(t *testing.T) {
	t.Parallel()
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

// realConflictFixture builds a fixture for verb ("cherry-pick" or "revert")
// across files ("conflict.txt" if none given) where both sides edit the
// same lines, so the call stops with a real content conflict rather than a
// dirty-worktree refusal.
func realConflictFixture(t *testing.T, dir, verb string, files ...string) (sha string) {
	t.Helper()
	if len(files) == 0 {
		files = []string{"conflict.txt"}
	}
	step := func(branch, from, msg, content string) gittest.Step {
		changed := map[string]string{}
		for _, f := range files {
			changed[f] = content
		}
		return gittest.Step{Branch: branch, From: from, Message: msg, Files: changed}
	}
	if verb == "cherry-pick" {
		shas := gittest.History(t, dir,
			step("master", "", "base", "orig\n"),
			step("feature", "master", "feature change", "feature\n"),
			step("master", "", "master change", "master\n"),
		)
		return shas[1]
	}
	shas := gittest.History(t, dir,
		step("master", "", "base", "base\n"),
		step("master", "", "applies the change", "applied\n"),
		step("master", "", "further change", "applied2\n"),
	)
	return shas[1]
}

// revertConflictFixture builds linear history where HEAD's commit (returned)
// changed conflict.txt, with an uncommitted, differing edit to the same file
// on top, so reverting HEAD's commit is refused as "would be overwritten"
// (exit 128).
func revertConflictFixture(t *testing.T, dir string) (headSHA string) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"conflict.txt": "base\n"}},
		gittest.Step{Message: "applies the change", Files: map[string]string{"conflict.txt": "applied\n"}},
	)
	writeFile(t, dir, "conflict.txt", "dirty")
	return shas[1]
}

// cherryPickConflictFixture builds a commit on branch "feature" that
// changes conflict.txt, while "master" has an uncommitted, conflicting edit
// to the same file, so cherry-picking feature's commit is refused as "would
// be overwritten" (exit 128).
func cherryPickConflictFixture(t *testing.T, dir string) (pickSHA, beforeHEAD string) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"conflict.txt": "base\n"}},
		gittest.Step{Branch: "feature", From: "master", Message: "feature change", Files: map[string]string{"conflict.txt": "feature\n"}},
	)
	writeFile(t, dir, "conflict.txt", "dirty")
	return shas[1], shas[0]
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
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"base.txt": "base"}},
		gittest.Step{Message: "x1", Files: map[string]string{"x1.txt": "x1"}},
		gittest.Step{Branch: "side", From: "master", Message: "side change", Files: map[string]string{"side.txt": "side"}},
		gittest.Step{Message: "diverging master commit", Files: map[string]string{"master2.txt": "master2"}},
		gittest.Step{Message: "Merge branch 'side'", Merge: "side", Files: map[string]string{"side.txt": "side"}},
		gittest.Step{Message: "x2", Files: map[string]string{"x2.txt": "x2"}},
	)
	return shas[1], shas[4], shas[5]
}

// commitDated commits like gittest.Commit, but at an explicit date: git's
// timestamps have 1s resolution, so two fast commits could otherwise tie.
func commitDated(t *testing.T, dir, name, content, msg string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "--", name)
	iso := at.Format(time.RFC3339)
	result, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir:   dir,
		Args:  []string{"commit", "--date=" + iso, "-F", "-"},
		Stdin: []byte(msg),
		Env:   []string{"GIT_COMMITTER_DATE=" + iso},
	})
	if err != nil {
		t.Fatalf("git commit: %v\n%s", err, result.Stderr)
	}
}
