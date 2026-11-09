// Requirements: FR-REPO-03, FR-CONF-01, FR-CONF-04, FR-CP-03, FR-CP-07, FR-BR-11, FR-BR-12, FR-LOG-14

package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
)

var everyOperation = []core.OperationKind{"", core.MergeOperation, core.RebaseOperation, core.CherryPickOperation, core.RevertOperation}

// keyOf is e's translation key, "" for nil.
func keyOf(e *core.Error) string {
	if e == nil {
		return ""
	}
	return e.Key
}

func TestIntegrateRefusesAnOperationInProgressBeforeADetachedHead(t *testing.T) {
	t.Parallel()
	for _, op := range everyOperation {
		for _, detached := range []bool{false, true} {
			want := ""
			switch {
			case op != "":
				want = "operationInProgress"
			case detached:
				want = "detachedHead"
			}
			got := integrateRefusal(op, detached)
			if keyOf(got) != want {
				t.Errorf("integrateRefusal(%q, detached %v) key = %q, want %q", op, detached, keyOf(got), want)
			}
			if got != nil && (got.Code != core.GitFailed || got.Args["operation"] != string(op)) {
				t.Errorf("integrateRefusal(%q, detached %v) = %+v, want GitFailed naming the operation", op, detached, got)
			}
		}
	}
}

func TestSquashNamesTheFirstBlockerInTheDocumentedOrder(t *testing.T) {
	t.Parallel()
	type state struct {
		op                                          core.OperationKind
		detached, notOnBranch, mergeInTheWay, dirty bool
	}
	cases := []struct {
		name string
		in   state
		want string
	}{
		{"nothing in the way", state{}, ""},
		{"an operation in progress", state{op: core.MergeOperation}, "squashOperationInProgress"},
		{"a detached HEAD", state{detached: true}, "squashDetached"},
		{"a commit not on the branch", state{notOnBranch: true}, "squashNotOnBranch"},
		{"a merge commit", state{mergeInTheWay: true}, "squashMergeInTheWay"},
		{"uncommitted changes", state{dirty: true}, "squashUncommittedChanges"},
		{"an operation before a detached HEAD", state{op: core.RebaseOperation, detached: true}, "squashOperationInProgress"},
		{"a detached HEAD before a commit not on the branch", state{detached: true, notOnBranch: true}, "squashDetached"},
		{"a commit not on the branch before a merge commit", state{notOnBranch: true, mergeInTheWay: true}, "squashNotOnBranch"},
		{"a merge commit before uncommitted changes", state{mergeInTheWay: true, dirty: true}, "squashMergeInTheWay"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := selection{
				chain:    []chainCommit{{sha: "a", short: "a", parents: []string{"base"}}, {sha: "b", short: "b", parents: []string{"a"}}},
				selected: map[string]bool{"a": true, "b": true},
			}
			if tc.in.notOnBranch {
				sel.selected["elsewhere"] = true
			}
			if tc.in.mergeInTheWay {
				sel.chain = append(sel.chain, chainCommit{sha: "m", short: "m", parents: []string{"b", "side"}})
			}
			st := headStatus{detached: tc.in.detached, dirty: tc.in.dirty}
			if got := keyOf(sel.firstBlocker(tc.in.op, st)); got != tc.want {
				t.Errorf("firstBlocker key = %q, want %q", got, tc.want)
			}
		})
	}
}

// An empty cherry-pick (no conflicted file) is Conflicted through
// remapIfPending, an empty fixup result Stopped through asStop: the two
// classifiers differ on purpose.
func TestAStopIsConflictedOrStoppedWhileAPendingFailureIsConflictedOnlyWhenGitFailed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code             core.Code
		conflicted       bool // asStop's conflicted and remapIfPending's pending
		asStop, remapped core.Code
	}{
		{core.GitFailed, false, core.Stopped, core.GitFailed},
		{core.GitFailed, true, core.Conflicted, core.Conflicted},
		{core.Cancelled, false, core.Stopped, core.Cancelled}, // a timeout
		{core.Cancelled, true, core.Conflicted, core.Cancelled},
	}
	for _, tc := range cases {
		if got := core.ErrorOf(asStop(core.Errorf(tc.code, "x"), tc.conflicted)).Code; got != tc.asStop {
			t.Errorf("asStop(%s, conflicted %v) = %s, want %s", tc.code, tc.conflicted, got, tc.asStop)
		}
		if got := core.ErrorOf(remapIfPending(core.Errorf(tc.code, "x"), tc.conflicted)).Code; got != tc.remapped {
			t.Errorf("remapIfPending(%s, pending %v) = %s, want %s", tc.code, tc.conflicted, got, tc.remapped)
		}
	}
	if err := remapIfPending(nil, true); err != nil {
		t.Errorf("remapIfPending(nil, pending) = %v, want nil", err)
	}
}

func TestRebasingBranchFallsBackToTheApplyDirectoryWhenTheMergeOneNamesNoBranch(t *testing.T) {
	t.Parallel()
	gitDir := t.TempDir()
	for dir, headName := range map[string]string{"rebase-merge": "detached HEAD\n", "rebase-apply": "refs/heads/topic\n"} {
		if err := os.MkdirAll(filepath.Join(gitDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(gitDir, dir), "head-name", headName)
	}

	if branch, ok := rebasingBranch(gitDir); branch != "topic" || !ok {
		t.Errorf("rebasingBranch = %q, %v; want topic, true", branch, ok)
	}
}

func TestOnlyACherryPickOrRevertIsSkippable(t *testing.T) {
	t.Parallel()
	for _, op := range everyOperation {
		want := op == core.CherryPickOperation || op == core.RevertOperation
		if got := skippable(op); got != want {
			t.Errorf("skippable(%q) = %v, want %v", op, got, want)
		}
	}
}

// rebaseStoppedAtBreak returns a repo whose master (base, then a change to
// f.txt) is stopped by a break in an interactive rebase onto base, and the
// tip of branch side, which also changed f.txt.
func rebaseStoppedAtBreak(t *testing.T) (dir string, side core.SHA) {
	t.Helper()
	dir = gittest.New(t).Dir
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"f.txt": "base\n"}},
		gittest.Step{Branch: "side", From: "master", Message: "side", Files: map[string]string{"f.txt": "side\n"}},
		gittest.Step{Message: "master", Files: map[string]string{"f.txt": "master\n"}},
	)
	// git runs the sequence editor as a shell command with the todo file appended.
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir: dir, Env: []string{"GIT_SEQUENCE_EDITOR=echo break >>"}, Args: []string{"rebase", "-i", "HEAD~1"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "REBASE_HEAD")); err == nil {
		t.Fatal("a break stop left REBASE_HEAD; the fixture no longer proves detection without it")
	}
	return dir, shas[1]
}

func TestSummaryReadsARebaseStoppedAtABreakAsARebase(t *testing.T) {
	t.Parallel()
	dir, _ := rebaseStoppedAtBreak(t)
	c := New(func(core.Event) {})

	assertOperation(t, c, openOn(t, c, dir).ID, core.RebaseOperation)
}

func TestSummaryReadsACherryPickStoppedInsideARebaseAsTheCherryPick(t *testing.T) {
	t.Parallel()
	dir, side := rebaseStoppedAtBreak(t)
	gitStops(t, dir, "cherry-pick", string(side))
	c := New(func(core.Event) {})

	assertOperation(t, c, openOn(t, c, dir).ID, core.CherryPickOperation)
}
