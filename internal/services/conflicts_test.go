package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// stoppedRepo opens a fresh Core on a repo stopped mid-kind ("merge",
// "rebase", "cherry-pick", "revert"), conflicted on files (conflictPath(kind)
// if none given). Merge and rebase stop via Pull, cherry-pick and revert via
// realConflictFixture. Returns HEAD before the operation.
func stoppedRepo(t *testing.T, kind string, files ...string) (dir string, c *Core, wt core.WorktreeID, beforeHEAD string) {
	t.Helper()
	c = New(func(core.Event) {})

	if kind == "merge" || kind == "rebase" {
		if len(files) == 0 {
			files = []string{"seed.txt"}
		}
		repo, origin := pullMergeFixture(t)
		for _, f := range files[1:] {
			gittest.Commit(t, repo.Dir, f, "base\n", "add "+f)
			gittest.Run(t, repo.Dir, "push", "origin", "master")
		}

		local, upstream := map[string]string{}, map[string]string{}
		for _, f := range files {
			local[f] = "local change\n"
			upstream[f] = "upstream change\n"
		}
		for name, content := range local {
			writeFile(t, repo.Dir, name, content)
		}
		gittest.Run(t, repo.Dir, "add", "-A")
		gittest.CommitAll(t, repo.Dir, "local commit")
		beforeHEAD = gittest.HeadSHA(t, repo.Dir)

		remoteAppendCommitFiles(t, origin, "", upstream, "upstream commit")

		info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
		if err != nil {
			t.Fatal(err)
		}
		mode := core.PullMerge
		if kind == "rebase" {
			mode = core.PullRebase
		}
		jobID, err := c.Pull(context.Background(), info.ID, mode)
		if err != nil {
			t.Fatal(err)
		}
		waitForJob(t, c, info.ID, func(j core.JobInfo) bool { return j.ID == jobID && j.State == "failed" })
		return repo.Dir, c, info.ID, beforeHEAD
	}

	repo := gittest.New(t)
	sha := realConflictFixture(t, repo.Dir, kind, files...)
	beforeHEAD = gittest.HeadSHA(t, repo.Dir)
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	var opErr error
	if kind == "cherry-pick" {
		opErr = c.CherryPick(context.Background(), info.ID, []string{sha})
	} else {
		opErr = c.Revert(context.Background(), info.ID, []string{sha})
	}
	if opErr == nil {
		t.Fatalf("%s: want a conflict, got nil error", kind)
	}
	return repo.Dir, c, info.ID, beforeHEAD
}

// conflictPath is the file stoppedRepo's own fixture conflicts on for kind.
func conflictPath(kind string) string {
	if kind == "merge" || kind == "rebase" {
		return "seed.txt"
	}
	return "conflict.txt"
}

var operationKinds = []string{"merge", "rebase", "cherry-pick", "revert"}

// assertOperationCompleted asserts what every kind's own successful
// continue leaves behind: HEAD advanced past beforeHEAD with the completed
// operation's own commit, and Summary.Operation cleared.
func assertOperationCompleted(t *testing.T, c *Core, wt core.WorktreeID, dir, beforeHEAD string) {
	t.Helper()
	if got := gittest.HeadSHA(t, dir); got == beforeHEAD {
		t.Error("HEAD did not advance with the completed operation's own commit")
	}
	summary, err := c.Summary(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Operation != "" {
		t.Errorf("Operation = %q, want cleared", summary.Operation)
	}
}

func TestFR_CONF_04_ContinueOperationSucceedsOnceTheOnlyConflictIsResolved(t *testing.T) {
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, beforeHEAD := stoppedRepo(t, kind)
			if err := c.StageFile(context.Background(), wt, conflictPath(kind)); err != nil {
				t.Fatal(err)
			}

			if err := c.ContinueOperation(context.Background(), wt); err != nil {
				t.Fatalf("ContinueOperation = %v, want nil", err)
			}

			assertOperationCompleted(t, c, wt, dir, beforeHEAD)
		})
	}
}

func TestFR_CONF_04_ContinueOperationRebaseNeverWaitsOnACommitMessageEditor(t *testing.T) {
	_, c, wt, _ := stoppedRepo(t, "rebase")
	if err := c.StageFile(context.Background(), wt, conflictPath("rebase")); err != nil {
		t.Fatal(err)
	}

	if err := c.ContinueOperation(context.Background(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil, no hang on a commit-message editor", err)
	}
}

// operationStateFile is the file (under .git) each kind's own stop leaves
// behind, and AbortOperation removes.
var operationStateFile = map[string]string{
	"merge":       "MERGE_HEAD",
	"rebase":      "rebase-merge",
	"cherry-pick": "CHERRY_PICK_HEAD",
	"revert":      "REVERT_HEAD",
}

func TestFR_CONF_04_AbortOperationRestoresTheFreshStop(t *testing.T) {
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, beforeHEAD := stoppedRepo(t, kind)

			if err := c.AbortOperation(context.Background(), wt); err != nil {
				t.Fatalf("AbortOperation = %v, want nil", err)
			}

			if fileExists(filepath.Join(dir, ".git", operationStateFile[kind])) {
				t.Errorf("%s present, want gone after abort", operationStateFile[kind])
			}
			if got := gittest.HeadSHA(t, dir); got != beforeHEAD {
				t.Errorf("HEAD = %s, want back at the pre-operation commit %s", got, beforeHEAD)
			}
			if got := currentBranch(t, dir); got != "master" {
				t.Errorf("current branch = %q, want master, not left detached", got)
			}
			status, err := c.Status(context.Background(), wt)
			if err != nil {
				t.Fatal(err)
			}
			if len(status.Conflicted) != 0 {
				t.Errorf("Conflicted = %v, want none after abort", status.Conflicted)
			}
		})
	}
}

func TestFR_CONF_04_ContinueAndAbortOperationRefuseWithNoOperationInProgress(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	c, wt := openWorktree(t, repo.Dir)

	before, err := c.CommandLog(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}

	coretest.AssertCode(t, c.ContinueOperation(context.Background(), wt), core.Internal)
	coretest.AssertCode(t, c.AbortOperation(context.Background(), wt), core.Internal)

	after, err := c.CommandLog(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("CommandLog grew from %d to %d entries, want unchanged (no git call made)", len(before), len(after))
	}
}

// unresolvedMessage is the substring git's refusal carries for each kind's
// --continue while a conflicted file remains. Rebase prints its refusal to
// stdout; GitStderr carries it via the runner's stdout fallback.
var unresolvedMessage = map[string]string{
	"merge":       "unresolved conflict",
	"rebase":      "needs merge",
	"cherry-pick": "unresolved conflict",
	"revert":      "unresolved conflict",
}

func TestFR_CONF_04_ContinueOperationRefusesWhileAConflictedFileRemains(t *testing.T) {
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, _ := stoppedRepo(t, kind, conflictPath(kind), "other.txt")
			// resolves only one of the two conflicted files
			if err := c.StageFile(context.Background(), wt, conflictPath(kind)); err != nil {
				t.Fatal(err)
			}

			err := c.ContinueOperation(context.Background(), wt)

			ce := coretest.AsError(t, err, core.Conflicts)
			if want := unresolvedMessage[kind]; !strings.Contains(ce.GitStderr, want) {
				t.Errorf("GitStderr = %q, want it to contain %q", ce.GitStderr, want)
			}
			if !fileExists(filepath.Join(dir, ".git", operationStateFile[kind])) {
				t.Errorf("%s missing, want it left in place (still stopped)", operationStateFile[kind])
			}
			summary, err := c.Summary(context.Background(), wt)
			if err != nil {
				t.Fatal(err)
			}
			if summary.Operation != kind {
				t.Errorf("Operation = %q, want still %q", summary.Operation, kind)
			}
		})
	}
}

func TestFR_CONF_01_SummaryAndStatusDetectEveryStoppedOperationKind(t *testing.T) {
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			_, c, wt, _ := stoppedRepo(t, kind)

			summary, err := c.Summary(context.Background(), wt)
			if err != nil {
				t.Fatal(err)
			}
			if summary.Operation != kind {
				t.Errorf("Operation = %q, want %q", summary.Operation, kind)
			}

			status, err := c.Status(context.Background(), wt)
			if err != nil {
				t.Fatal(err)
			}
			want := core.ChangedFile{Status: "UU", Path: conflictPath(kind)}
			if !reflect.DeepEqual(status.Conflicted, []core.ChangedFile{want}) {
				t.Errorf("Conflicted = %+v, want [%+v] (git's own raw XY code)", status.Conflicted, want)
			}
		})
	}
}

func TestFR_CONF_03_StageFileResolvesAnOrdinaryConflictAndContinueCompletes(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "conflict.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "conflict.txt", "feature\n", "feature change")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master\n", "master change")

	c, wt := openWorktree(t, repo.Dir)
	// Real merge conflict, direct via the runner: leaves MERGE_HEAD and an
	// unmerged conflict.txt (UU, both modified).
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{
		Dir: repo.Dir, Args: []string{"merge", "--end-of-options", "feature"},
	})
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}
	status, err := c.Status(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if want := (core.ChangedFile{Status: "UU", Path: "conflict.txt"}); !reflect.DeepEqual(status.Conflicted, []core.ChangedFile{want}) {
		t.Fatalf("Conflicted = %+v, want [%+v]", status.Conflicted, want)
	}

	writeFile(t, repo.Dir, "conflict.txt", "resolved\n") // edited in place, markers gone

	if err := c.StageFile(context.Background(), wt, "conflict.txt"); err != nil {
		t.Fatalf("StageFile = %v, want nil", err)
	}

	status, err = c.Status(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Conflicted) != 0 {
		t.Errorf("Conflicted = %v, want none after StageFile", status.Conflicted)
	}
	if want := (core.ChangedFile{Status: "M", Path: "conflict.txt"}); !reflect.DeepEqual(status.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", status.Staged, want)
	}

	if err := c.ContinueOperation(context.Background(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}
	summary, err := c.Summary(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Operation != "" {
		t.Errorf("Operation = %q, want cleared", summary.Operation)
	}
}

func TestFR_CONF_03_StageFileResolvesADeleteConflictAndContinueCompletes(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "shared.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Run(t, repo.Dir, "rm", "shared.txt")
	gittest.CommitAll(t, repo.Dir, "delete shared.txt")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "shared.txt", "master change\n", "master change") // ours modifies; theirs (feature) deletes

	c, wt := openWorktree(t, repo.Dir)
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{
		Dir: repo.Dir, Args: []string{"merge", "--end-of-options", "feature"},
	})
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}
	status, err := c.Status(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Conflicted) != 1 || status.Conflicted[0].Path != "shared.txt" {
		t.Fatalf("Conflicted = %+v, want one entry for shared.txt", status.Conflicted)
	}

	if err := os.Remove(filepath.Join(repo.Dir, "shared.txt")); err != nil {
		t.Fatal(err)
	}

	if err := c.StageFile(context.Background(), wt, "shared.txt"); err != nil {
		t.Fatalf("StageFile = %v, want nil (accepts the deletion)", err)
	}

	status, err = c.Status(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Conflicted) != 0 {
		t.Errorf("Conflicted = %v, want none after StageFile", status.Conflicted)
	}
	if want := (core.ChangedFile{Status: "D", Path: "shared.txt"}); !reflect.DeepEqual(status.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v] (a staged deletion)", status.Staged, want)
	}

	if err := c.ContinueOperation(context.Background(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}
	summary, err := c.Summary(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Operation != "" {
		t.Errorf("Operation = %q, want cleared", summary.Operation)
	}
}

func TestFR_CP_03_AStoppedCherryPickResolvesAndCompletesThroughContinueOperation(t *testing.T) {
	dir, c, wt, beforeHEAD := stoppedRepo(t, "cherry-pick")

	if err := c.StageFile(context.Background(), wt, conflictPath("cherry-pick")); err != nil {
		t.Fatal(err)
	}
	if err := c.ContinueOperation(context.Background(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD present, want gone")
	}
	assertOperationCompleted(t, c, wt, dir, beforeHEAD)
}

func TestFR_RV_03_AStoppedRevertResolvesAndCompletesThroughContinueOperation(t *testing.T) {
	dir, c, wt, beforeHEAD := stoppedRepo(t, "revert")

	if err := c.StageFile(context.Background(), wt, conflictPath("revert")); err != nil {
		t.Fatal(err)
	}
	if err := c.ContinueOperation(context.Background(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD present, want gone")
	}
	assertOperationCompleted(t, c, wt, dir, beforeHEAD)
}
