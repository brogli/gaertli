// Requirements: FR-CONF-01, FR-CONF-03, FR-CONF-04, FR-CP-03, FR-CP-07, FR-RV-03, QA-COMP-01

package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
)

// stoppedRepo opens a fresh Core on a repo stopped mid-kind ("merge",
// "rebase", "cherry-pick", "revert"), conflicted on files (conflictPath(kind)
// if none given). Merge and rebase stop on a local diverged branch, cherry-pick
// and revert via realConflictFixture. Returns HEAD before the operation.
func stoppedRepo(t *testing.T, kind string, files ...string) (dir string, c *Core, wt core.WorktreeID, beforeHEAD core.SHA) {
	t.Helper()
	c = New(func(core.Event) {})

	if kind == "merge" || kind == "rebase" {
		if len(files) == 0 {
			files = []string{"seed.txt"}
		}
		return stoppedByLocalOperation(t, c, kind, files)
	}

	repo := gittest.New(t)
	sha := realConflictFixture(t, repo.Dir, kind, files...)
	beforeHEAD = gittest.HeadSHA(t, repo.Dir)
	info := openOn(t, c, repo.Dir)
	var opErr error
	if kind == "cherry-pick" {
		opErr = c.CherryPick(t.Context(), info.ID, []core.SHA{sha}, core.PickOptions{})
	} else {
		opErr = c.Revert(t.Context(), info.ID, []core.SHA{sha}, core.PickOptions{})
	}
	if opErr == nil {
		t.Fatalf("%s: want a conflict, got nil error", kind)
	}
	return repo.Dir, c, info.ID, beforeHEAD
}

// stoppedByLocalOperation merges or rebases branch side into master, where
// both changed every file.
func stoppedByLocalOperation(t *testing.T, c *Core, kind string, files []string) (string, *Core, core.WorktreeID, core.SHA) {
	t.Helper()
	dir := gittest.New(t).Dir
	contents := func(content string) map[string]string {
		m := map[string]string{}
		for _, f := range files {
			m[f] = content
		}
		return m
	}
	gittest.History(t, dir,
		gittest.Step{Message: "base", Files: contents("base\n")},
		gittest.Step{Branch: "side", From: "master", Message: "upstream commit", Files: contents("upstream change\n")},
		gittest.Step{Message: "local commit", Files: contents("local change\n")},
	)
	beforeHEAD := gittest.HeadSHA(t, dir)
	gitStops(t, dir, kind, "side")
	info := openOn(t, c, dir)
	return dir, c, info.ID, beforeHEAD
}

// conflictPath is the file stoppedRepo's own fixture conflicts on for kind.
func conflictPath(kind string) string {
	if kind == "merge" || kind == "rebase" {
		return "seed.txt"
	}
	return "conflict.txt"
}

var operationKinds = []string{"merge", "rebase", "cherry-pick", "revert"}

// assertOperation asserts that Summary names want as the operation in
// progress, "" for none.
func assertOperation(t *testing.T, c *Core, wt core.WorktreeID, want core.Operation) {
	t.Helper()
	summary, err := c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Operation != want {
		t.Errorf("Operation = %q, want %q", summary.Operation, want)
	}
}

func TestContinueOperationSucceedsOnceTheOnlyConflictIsResolved(t *testing.T) {
	t.Parallel()
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, beforeHEAD := stoppedRepo(t, kind)
			if err := c.StageFile(t.Context(), wt, conflictPath(kind)); err != nil {
				t.Fatal(err)
			}

			if err := c.ContinueOperation(t.Context(), wt); err != nil {
				t.Fatalf("ContinueOperation = %v, want nil, no hang on a commit-message editor", err)
			}

			if fileExists(filepath.Join(dir, ".git", operationStateFile[kind])) {
				t.Errorf("%s present, want gone once the operation completed", operationStateFile[kind])
			}
			if got := gittest.HeadSHA(t, dir); got == beforeHEAD {
				t.Error("HEAD did not advance with the completed operation's own commit")
			}
			assertOperation(t, c, wt, "")
		})
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

func TestAbortOperationRestoresTheFreshStop(t *testing.T) {
	t.Parallel()
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, beforeHEAD := stoppedRepo(t, kind)

			if err := c.AbortOperation(t.Context(), wt); err != nil {
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
			status := statusOf(t, c, wt)
			if len(status.Conflicted) != 0 {
				t.Errorf("Conflicted = %v, want none after abort", status.Conflicted)
			}
		})
	}
}

// An abort never commits, so it runs in the user's editor environment.
// Every abort runs the reference-transaction hook, which sees the
// environment the call got.
func TestAbortOperationKeepsTheUsersEditorEnvironment(t *testing.T) {
	unsetEditorEnv(t)
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, _ := stoppedRepo(t, kind)
			gittest.Run(t, dir, "config", "core.editor", "false")
			seen := filepath.ToSlash(filepath.Join(t.TempDir(), "editor"))
			writeHook(t, dir, "reference-transaction", `echo "${GIT_EDITOR-unset}" >> "`+seen+`"`+"\n")

			if err := c.AbortOperation(t.Context(), wt); err != nil {
				t.Fatalf("AbortOperation = %v, want nil", err)
			}

			got := strings.Fields(readFile(t, filepath.Dir(seen), "editor"))
			if len(got) == 0 || slices.ContainsFunc(got, func(v string) bool { return v != "unset" }) {
				t.Errorf("GIT_EDITOR seen by the hook = %q, want unset every time", got)
			}
		})
	}
}

// A sequence started outside the app with --edit keeps that choice: a
// skip commits each remaining commit with a message, and must not wait on
// the user's editor for it.
func TestSkipOperationNeverWaitsOnAnEditorTheSequenceAsksFor(t *testing.T) {
	unsetEditorEnv(t)
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, "c.txt", "base\n", "base")
	gittest.Commit(t, dir, "one.txt", "1\n", "one")
	one := gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "c.txt", "v2\n", "c2")
	conflicting := gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "c.txt", "v3\n", "c3")
	gittest.Commit(t, dir, "three.txt", "3\n", "three")
	three := gittest.HeadSHA(t, dir)
	// Newest first: reverting c2 conflicts with c3.
	tryGit(t, dir, "-c", "core.editor=true", "revert", "--edit", string(three), string(conflicting), string(one))
	if !fileExists(filepath.Join(dir, ".git", "REVERT_HEAD")) {
		t.Fatal("fixture did not stop on the conflicting revert")
	}
	gittest.Run(t, dir, "config", "core.editor", "false")
	c, wt := openWorktree(t, dir)

	if err := c.SkipOperation(t.Context(), wt); err != nil {
		t.Fatalf("SkipOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "sequencer")) {
		t.Error(".git/sequencer present, want the sequence finished")
	}
	if got := gittest.Out(t, dir, "log", "-1", "--format=%s"); got != `Revert "one"` {
		t.Errorf("last commit = %q, want the remaining revert committed", got)
	}
}

// unsetEditorEnv clears the editor variables for the test, so only the
// repo's config or the call's own environment can pick an editor.
func unsetEditorEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{"GIT_EDITOR", "GIT_SEQUENCE_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(v, "")
		if err := os.Unsetenv(v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestContinueAbortAndSkipOperationSayNoOperationIsInProgress(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	for _, verb := range []func(context.Context, core.WorktreeID) error{c.ContinueOperation, c.AbortOperation, c.SkipOperation} {
		if ce := coretest.AsError(t, verb(t.Context(), wt), core.GitFailed); ce.Key != "noOperation" {
			t.Errorf("Key = %q, want noOperation", ce.Key)
		}
	}

	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("git calls = %d, want 0 (no git call made)", got)
	}
}

func TestContinueOperationRefusesWhileAConflictedFileRemains(t *testing.T) {
	t.Parallel()
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			dir, c, wt, _ := stoppedRepo(t, kind, conflictPath(kind), "other.txt")
			// resolves only one of the two conflicted files
			if err := c.StageFile(t.Context(), wt, conflictPath(kind)); err != nil {
				t.Fatal(err)
			}

			err := c.ContinueOperation(t.Context(), wt)

			ce := coretest.AsError(t, err, core.Conflicted)
			if ce.GitStderr == "" {
				t.Error("GitStderr empty, want git's own explanation")
			}
			if !fileExists(filepath.Join(dir, ".git", operationStateFile[kind])) {
				t.Errorf("%s missing, want it left in place (still stopped)", operationStateFile[kind])
			}
			assertOperation(t, c, wt, core.Operation(kind))
		})
	}
}

func TestAContinueThatStopsForAnotherReasonThanAConflictIsAStop(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	l := lineFixture(t, repo.Dir)
	writeHook(t, repo.Dir, "prepare-commit-msg", refuseAmendHook) // fails the squash's amend step
	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Squash(t.Context(), wt, []core.SHA{l.B, l.D}, "Folded")
	coretest.AssertCode(t, err, core.Stopped)

	err = c.ContinueOperation(t.Context(), wt)

	ce := coretest.AsError(t, err, core.Stopped)
	if !strings.Contains(ce.GitStderr, "refusing") {
		t.Errorf("GitStderr = %q, want the hook's reason", ce.GitStderr)
	}
	if operation(filepath.Join(repo.Dir, ".git")) != "rebase" {
		t.Error("no rebase in progress, want it still stopped")
	}
}

func TestSummaryAndStatusDetectEveryStoppedOperationKind(t *testing.T) {
	t.Parallel()
	for _, kind := range operationKinds {
		t.Run(kind, func(t *testing.T) {
			_, c, wt, _ := stoppedRepo(t, kind)

			assertOperation(t, c, wt, core.Operation(kind))

			status := statusOf(t, c, wt)
			want := core.ChangedFile{Status: "UU", Path: conflictPath(kind)}
			if !reflect.DeepEqual(status.Conflicted, []core.ChangedFile{want}) {
				t.Errorf("Conflicted = %+v, want [%+v] (git's own raw XY code)", status.Conflicted, want)
			}
		})
	}
}

func TestStageFileResolvesAnOrdinaryConflictAndContinueCompletes(t *testing.T) {
	t.Parallel()
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
	tryGit(t, repo.Dir, "merge", "--end-of-options", "feature")
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}
	status := statusOf(t, c, wt)
	if want := (core.ChangedFile{Status: "UU", Path: "conflict.txt"}); !reflect.DeepEqual(status.Conflicted, []core.ChangedFile{want}) {
		t.Fatalf("Conflicted = %+v, want [%+v]", status.Conflicted, want)
	}

	writeFile(t, repo.Dir, "conflict.txt", "resolved\n") // edited in place, markers gone

	if err := c.StageFile(t.Context(), wt, "conflict.txt"); err != nil {
		t.Fatalf("StageFile = %v, want nil", err)
	}

	status = statusOf(t, c, wt)
	if len(status.Conflicted) != 0 {
		t.Errorf("Conflicted = %v, want none after StageFile", status.Conflicted)
	}
	if want := (core.ChangedFile{Status: "M", Path: "conflict.txt"}); !reflect.DeepEqual(status.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", status.Staged, want)
	}

	if err := c.ContinueOperation(t.Context(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}
	assertOperation(t, c, wt, "")
}

func TestStageFileResolvesADeleteConflictAndContinueCompletes(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "shared.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Run(t, repo.Dir, "rm", "shared.txt")
	gittest.CommitAll(t, repo.Dir, "delete shared.txt")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "shared.txt", "master change\n", "master change") // ours modifies; theirs (feature) deletes

	c, wt := openWorktree(t, repo.Dir)
	tryGit(t, repo.Dir, "merge", "--end-of-options", "feature")
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}
	status := statusOf(t, c, wt)
	if len(status.Conflicted) != 1 || status.Conflicted[0].Path != "shared.txt" {
		t.Fatalf("Conflicted = %+v, want one entry for shared.txt", status.Conflicted)
	}

	if err := os.Remove(filepath.Join(repo.Dir, "shared.txt")); err != nil {
		t.Fatal(err)
	}

	if err := c.StageFile(t.Context(), wt, "shared.txt"); err != nil {
		t.Fatalf("StageFile = %v, want nil (accepts the deletion)", err)
	}

	status = statusOf(t, c, wt)
	if len(status.Conflicted) != 0 {
		t.Errorf("Conflicted = %v, want none after StageFile", status.Conflicted)
	}
	if want := (core.ChangedFile{Status: "D", Path: "shared.txt"}); !reflect.DeepEqual(status.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v] (a staged deletion)", status.Staged, want)
	}

	if err := c.ContinueOperation(t.Context(), wt); err != nil {
		t.Fatalf("ContinueOperation = %v, want nil", err)
	}
	assertOperation(t, c, wt, "")
}

func TestSkipOperationCherryPickDropsTheConflictingCommitAndAppliesTheSurvivor(t *testing.T) {
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, "conflict.txt", "orig\n", "base")
	gittest.Run(t, dir, "branch", "feature")
	gittest.Run(t, dir, "checkout", "feature")
	gittest.Commit(t, dir, "conflict.txt", "feature\n", "feature change")
	conflictSHA := gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "other.txt", "other\n", "clean change")
	cleanSHA := gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "checkout", "master")
	gittest.Commit(t, dir, "conflict.txt", "master\n", "master change")

	unsetEditorEnv(t)
	gittest.Run(t, dir, "config", "core.editor", "false") // the rest of the sequence needs no editor
	c, wt := openWorktree(t, dir)
	// oldest-first: conflictSHA (older) is attempted before cleanSHA.
	if err := c.CherryPick(t.Context(), wt, []core.SHA{cleanSHA, conflictSHA}, core.PickOptions{}); err == nil {
		t.Fatal("CherryPick = nil, want a conflict on conflictSHA")
	}
	if !fileExists(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Fatal("fixture did not leave CHERRY_PICK_HEAD")
	}

	if err := c.SkipOperation(t.Context(), wt); err != nil {
		t.Fatalf("SkipOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD present, want gone once the sequence completed")
	}
	assertOperation(t, c, wt, "")
	if _, statErr := os.Stat(filepath.Join(dir, "other.txt")); statErr != nil {
		t.Errorf("other.txt missing after skip, want cleanSHA's own pick applied: %v", statErr)
	}
	content := readFile(t, dir, "conflict.txt")
	if content == "feature\n" {
		t.Error("conflict.txt carries the skipped commit's own change, want it absent")
	}
}

func TestSkipOperationRevertDropsTheConflictingRevertAndAppliesTheSurvivor(t *testing.T) {
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, dir, "other.txt", "obase\n", "other base")
	gittest.Commit(t, dir, "other.txt", "applied\n", "older change")
	cleanSHA := gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "conflict.txt", "applied\n", "newer change")
	conflictSHA := gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "conflict.txt", "applied2\n", "further change on top")

	unsetEditorEnv(t)
	gittest.Run(t, dir, "config", "core.editor", "false") // the rest of the sequence needs no editor
	c, wt := openWorktree(t, dir)
	// newest-first: conflictSHA (newer) is attempted before cleanSHA.
	if err := c.Revert(t.Context(), wt, []core.SHA{cleanSHA, conflictSHA}, core.PickOptions{}); err == nil {
		t.Fatal("Revert = nil, want a conflict on conflictSHA")
	}
	if !fileExists(filepath.Join(dir, ".git", "REVERT_HEAD")) {
		t.Fatal("fixture did not leave REVERT_HEAD")
	}

	if err := c.SkipOperation(t.Context(), wt); err != nil {
		t.Fatalf("SkipOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD present, want gone once the sequence completed")
	}
	assertOperation(t, c, wt, "")
	content := readFile(t, dir, "other.txt")
	if content != "obase\n" {
		t.Errorf("other.txt = %q, want reverted back to obase (cleanSHA's own revert)", content)
	}
	conflictContent := readFile(t, dir, "conflict.txt")
	if conflictContent != "applied2\n" {
		t.Errorf("conflict.txt = %q, want unchanged (the skipped revert's own change is absent)", conflictContent)
	}
}

func TestSkipOperationOnTheLastPendingCommitClearsTheOperationWithNothingLeftToApply(t *testing.T) {
	t.Parallel()
	dir, c, wt, beforeHEAD := stoppedRepo(t, "cherry-pick")

	if err := c.SkipOperation(t.Context(), wt); err != nil {
		t.Fatalf("SkipOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD")) {
		t.Error("CHERRY_PICK_HEAD present, want gone")
	}
	if got := gittest.HeadSHA(t, dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the skipped commit was the whole sequence)", got, beforeHEAD)
	}
	assertOperation(t, c, wt, "")
}

func TestSkipOperationRevertOnTheLastPendingCommitClearsTheOperationWithNothingLeftToApply(t *testing.T) {
	t.Parallel()
	dir, c, wt, beforeHEAD := stoppedRepo(t, "revert")

	if err := c.SkipOperation(t.Context(), wt); err != nil {
		t.Fatalf("SkipOperation = %v, want nil", err)
	}

	if fileExists(filepath.Join(dir, ".git", "REVERT_HEAD")) {
		t.Error("REVERT_HEAD present, want gone")
	}
	if got := gittest.HeadSHA(t, dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s (the skipped commit was the whole sequence)", got, beforeHEAD)
	}
	assertOperation(t, c, wt, "")
}

func TestSkipOperationMidMergeSaysAMergeCannotBeSkipped(t *testing.T) {
	t.Parallel()
	dir, c, wt, _ := stoppedRepo(t, "merge")
	rec := recordCommands(t, c, wt)

	err := c.SkipOperation(t.Context(), wt)

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Key != "operationNotSkippable" || ce.Args["operation"] != "merge" {
		t.Errorf("key, args = %q, %v; want operationNotSkippable, operation=merge", ce.Key, ce.Args)
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("git calls = %d, want 0 (no git call made)", got)
	}
	if !fileExists(filepath.Join(dir, ".git", "MERGE_HEAD")) {
		t.Error("MERGE_HEAD gone, want the merge left untouched")
	}
}
