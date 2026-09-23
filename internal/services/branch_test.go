package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_BR_05_RemapDirtyClassifiesByExitCodeNotText(t *testing.T) {
	tests := []struct {
		name string
		err  *core.Error
		exit int
		want core.Code
	}{
		{"GitFailed at exit 1 becomes DirtyWorktree", &core.Error{Code: core.GitFailed}, 1, core.DirtyWorktree},
		{"GitFailed at exit 128 stays GitFailed", &core.Error{Code: core.GitFailed}, 128, core.GitFailed},
		{"Cancelled at exit 1 stays Cancelled", &core.Error{Code: core.Cancelled}, 1, core.Cancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remapDirty(tt.err, git.Result{ExitCode: tt.exit})

			var ce *core.Error
			if !errors.As(got, &ce) || ce.Code != tt.want {
				t.Errorf("remapDirty(...) Code = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestFR_BR_02_ResetFlagMapsModeToGitFlag(t *testing.T) {
	tests := []struct {
		mode core.ResetMode
		want string
	}{
		{core.ResetSoft, "--soft"},
		{core.ResetMixed, "--mixed"},
		{core.ResetHard, "--hard"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			got, err := resetFlag(tt.mode)
			if err != nil {
				t.Fatalf("resetFlag(%q) error = %v, want nil", tt.mode, err)
			}
			if got != tt.want {
				t.Errorf("resetFlag(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}

	_, err := resetFlag(core.ResetMode("bogus"))
	coretest.AssertCode(t, err, core.Internal)
}

func TestFR_BR_06_RenameBranchRenamesANonCurrentBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	featureTip := branchTip(t, repo.Dir, "feature")
	mainTip := branchTip(t, repo.Dir, "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.RenameBranch(context.Background(), wt, "feature", "renamed"); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "renamed"); got != featureTip {
		t.Errorf("renamed = %s, want %s", got, featureTip)
	}
	if branchExists(t, repo.Dir, "feature") {
		t.Error("feature still exists, want renamed away")
	}
	if got := branchTip(t, repo.Dir, "master"); got != mainTip {
		t.Errorf("master = %s, want unchanged %s", got, mainTip)
	}
}

func TestFR_BR_06_RenameBranchRejectsAnInvalidNewNameBeforeTouchingGit(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	featureTip := branchTip(t, repo.Dir, "feature")

	c, wt := openWorktree(t, repo.Dir)

	err := c.RenameBranch(context.Background(), wt, "feature", "-badname")

	coretest.AssertCode(t, err, core.InvalidName)
	if got := branchTip(t, repo.Dir, "feature"); got != featureTip {
		t.Errorf("feature = %s, want unchanged %s (unrenamed)", got, featureTip)
	}
}

func TestFR_BR_06_RenameBranchRejectsADuplicateNewNameAsGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "branch", "taken")
	featureTip := branchTip(t, repo.Dir, "feature")
	takenTip := branchTip(t, repo.Dir, "taken")

	c, wt := openWorktree(t, repo.Dir)

	err := c.RenameBranch(context.Background(), wt, "feature", "taken")

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "already exists") {
		t.Errorf("GitStderr = %q, want it to say the branch already exists", ce.GitStderr)
	}
	if got := branchTip(t, repo.Dir, "feature"); got != featureTip {
		t.Errorf("feature = %s, want unchanged %s", got, featureTip)
	}
	if got := branchTip(t, repo.Dir, "taken"); got != takenTip {
		t.Errorf("taken = %s, want unchanged %s", got, takenTip)
	}
}

func TestFR_BR_06_DeleteBranchDeletesAFullyMergedBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "merged")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DeleteBranch(context.Background(), wt, "merged", false); err != nil {
		t.Fatal(err)
	}

	if branchExists(t, repo.Dir, "merged") {
		t.Error("merged still exists, want deleted")
	}
}

func TestFR_BR_06_DeleteBranchRefusesAnUnmergedBranchAsCodeUnmerged(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "topic")
	gittest.Run(t, repo.Dir, "checkout", "topic")
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	err := c.DeleteBranch(context.Background(), wt, "topic", false)

	coretest.AssertCode(t, err, core.Unmerged)
	if !branchExists(t, repo.Dir, "topic") {
		t.Error("topic gone, want still present (unforced delete refuses)")
	}
}

func TestFR_BR_06_DeleteBranchForcedDeletesAnUnmergedBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "topic")
	gittest.Run(t, repo.Dir, "checkout", "topic")
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DeleteBranch(context.Background(), wt, "topic", true); err != nil {
		t.Fatal(err)
	}

	if branchExists(t, repo.Dir, "topic") {
		t.Error("topic still exists, want deleted")
	}
}

func TestFR_BR_06_DeleteBranchRefusesABranchCheckedOutInAnotherWorktreeAsGitFailedNotUnmerged(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "other")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "other")

	c, wt := openWorktree(t, repo.Dir)

	err := c.DeleteBranch(context.Background(), wt, "other", false)
	coretest.AssertCode(t, err, core.GitFailed)

	err = c.DeleteBranch(context.Background(), wt, "other", true)
	coretest.AssertCode(t, err, core.GitFailed)

	if !branchExists(t, repo.Dir, "other") {
		t.Error("other gone, want still present")
	}
	if got := currentBranch(t, linked); got != "other" {
		t.Errorf("linked worktree's branch = %s, want still other", got)
	}
}

func TestFR_BR_07_ResetFlagMapsKeepToGitFlag(t *testing.T) {
	got, err := resetFlag(core.ResetKeep)
	if err != nil {
		t.Fatalf("resetFlag(keep) error = %v, want nil", err)
	}
	if got != "--keep" {
		t.Errorf("resetFlag(keep) = %q, want \"--keep\"", got)
	}
}

func TestFR_BR_01_CreateBranchWithoutCheckoutLeavesCurrentBranchAlone(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(context.Background(), wt, "feature", firstSHA, false); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "feature"); got != firstSHA {
		t.Errorf("feature = %s, want %s", got, firstSHA)
	}
	if got := currentBranch(t, repo.Dir); got != "master" {
		t.Errorf("current branch = %s, want master (unchanged)", got)
	}
}

func TestFR_BR_01_CreateBranchWithCheckoutSwitchesToIt(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(context.Background(), wt, "feature", firstSHA, true); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "feature"); got != firstSHA {
		t.Errorf("feature = %s, want %s", got, firstSHA)
	}
	if got := currentBranch(t, repo.Dir); got != "feature" {
		t.Errorf("current branch = %s, want feature", got)
	}
}

func TestFR_BR_01_CreateBranchRejectsADuplicateNameAsGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	originalTip := branchTip(t, repo.Dir, "feature")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(context.Background(), wt, "feature", sha, false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "already exists") {
		t.Errorf("GitStderr = %q, want it to say the branch already exists", ce.GitStderr)
	}
	if got := branchTip(t, repo.Dir, "feature"); got != originalTip {
		t.Errorf("feature = %s, want unchanged %s", got, originalTip)
	}
}

func TestFR_BR_01_CreateBranchRejectsAnInvalidNameBeforeTouchingGit(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(context.Background(), wt, "-bad", sha, false)

	coretest.AssertCode(t, err, core.InvalidName)
	if branchExists(t, repo.Dir, "-bad") {
		t.Error("branch -bad was created, want none")
	}
}

func TestFR_BR_01_CreateBranchWithCheckoutIsAtomicOnADirtyConflict(t *testing.T) {
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(context.Background(), wt, "old", olderSHA, true)

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if branchExists(t, repo.Dir, "old") {
		t.Error("branch old was created despite the atomic checkout failing, want none")
	}
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
	if got, _ := os.ReadFile(filepath.Join(repo.Dir, "conflict.txt")); string(got) != "dirty" {
		t.Errorf("conflict.txt = %q, want the uncommitted content unchanged", got)
	}
}

func TestFR_BR_05_CheckoutSwitchesToAnExistingLocalBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Checkout(context.Background(), wt, "feature"); err != nil {
		t.Fatal(err)
	}

	if got := currentBranch(t, repo.Dir); got != "feature" {
		t.Errorf("current branch = %s, want feature", got)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "seed.txt")); err != nil {
		t.Errorf("working tree missing feature's own file: %v", err)
	}
}

func TestFR_BR_05_CheckoutDWIMsARemoteTrackingBranchIntoALocalOne(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	bareDir := t.TempDir()
	gittest.Run(t, bareDir, "init", "--bare")
	gittest.Run(t, repo.Dir, "remote", "add", "origin", bareDir)
	gittest.Run(t, repo.Dir, "push", "origin", "master:shared")
	gittest.Run(t, repo.Dir, "fetch", "origin")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Checkout(context.Background(), wt, "shared"); err != nil {
		t.Fatal(err)
	}

	if got := currentBranch(t, repo.Dir); got != "shared" {
		t.Errorf("current branch = %s, want shared", got)
	}
	if got := gittest.Out(t, repo.Dir, "for-each-ref", "--format=%(upstream:short)", "refs/heads/shared"); got != "origin/shared" {
		t.Errorf("upstream = %q, want origin/shared", got)
	}
}

func TestFR_BR_05_CheckoutRefusesADirtyConflictAsDirtyWorktree(t *testing.T) {
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature", olderSHA) // a plain ref create, unaffected by the dirty working tree
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Checkout(context.Background(), wt, "feature")

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
}

func TestFR_BR_05_CheckoutNamesTheOtherWorktreeWhenABranchIsAlreadyCheckedOutThere(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Checkout(context.Background(), wt, "feature")

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, linked) {
		t.Errorf("GitStderr = %q, want it to name the other worktree's path %q", ce.GitStderr, linked)
	}
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
}

func TestFR_BR_08_CheckoutCommitDetachesHEADAtTheGivenSHA(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CheckoutCommit(context.Background(), wt, firstSHA); err != nil {
		t.Fatal(err)
	}

	if branch := currentBranch(t, repo.Dir); branch != "" {
		t.Errorf("current branch = %s, want detached (no branch)", branch)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != firstSHA {
		t.Errorf("HEAD = %s, want %s", got, firstSHA)
	}
}

func TestFR_BR_08_CheckoutCommitRefusesADirtyConflictAsDirtyWorktree(t *testing.T) {
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CheckoutCommit(context.Background(), wt, olderSHA)

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_08_CheckoutCommitSucceedsEvenWhenAlreadyDetachedInAnotherWorktree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linked, sha)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CheckoutCommit(context.Background(), wt, sha); err != nil {
		t.Errorf("CheckoutCommit = %v, want success (a detached commit may be shared across worktrees)", err)
	}
}

func TestQA_REL_02_BranchActionsFailCleanlyOnAnEmptyRepo(t *testing.T) {
	repo := gittest.New(t) // no commit: an unborn HEAD

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(context.Background(), wt, "feature", "HEAD", false); err == nil {
		t.Error("CreateBranch on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}

	if err := c.Checkout(context.Background(), wt, "anything"); err == nil {
		t.Error("Checkout on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}

	if err := c.CheckoutCommit(context.Background(), wt, "HEAD"); err == nil {
		t.Error("CheckoutCommit on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}
}

func TestFR_BR_02_ResetSoftMovesBranchLeavesIndexAndWorkingTreeUntouched(t *testing.T) {
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")
	beforeIndex := indexBlob(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(context.Background(), wt, firstSHA, core.ResetSoft); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "master"); got != firstSHA {
		t.Errorf("master = %s, want %s", got, firstSHA)
	}
	if got := readFile(t, repo.Dir, "seed.txt"); got != beforeWorking {
		t.Errorf("working tree seed.txt = %q, want unchanged %q", got, beforeWorking)
	}
	if got := indexBlob(t, repo.Dir, "seed.txt"); got != beforeIndex {
		t.Errorf("index seed.txt = %q, want unchanged %q", got, beforeIndex)
	}
}

func TestFR_BR_02_ResetMixedLeavesWorkingTreeUntracksPathsAbsentFromTarget(t *testing.T) {
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(context.Background(), wt, firstSHA, core.ResetMixed); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "master"); got != firstSHA {
		t.Errorf("master = %s, want %s", got, firstSHA)
	}
	if got := readFile(t, repo.Dir, "seed.txt"); got != beforeWorking {
		t.Errorf("working tree seed.txt = %q, want unchanged %q", got, beforeWorking)
	}
	if trackedInIndex(t, repo.Dir, "extra.txt") {
		t.Error("extra.txt still in the index, want untracked (absent from firstSHA's own tree)")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "extra.txt")); err != nil {
		t.Errorf("extra.txt missing from the working tree: %v", err)
	}
}

func TestFR_BR_02_ResetHardMatchesTargetExactlyLeavesUntrackedFileAlone(t *testing.T) {
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(context.Background(), wt, firstSHA, core.ResetHard); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "master"); got != firstSHA {
		t.Errorf("master = %s, want %s", got, firstSHA)
	}
	if got := readFile(t, repo.Dir, "seed.txt"); got != "a" {
		t.Errorf("seed.txt = %q, want firstSHA's own content %q", got, "a")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "extra.txt")); !os.IsNotExist(err) {
		t.Errorf("extra.txt = %v, want removed (absent from firstSHA's own tree)", err)
	}
	if got := readFile(t, repo.Dir, "untracked.txt"); got != "keep me" {
		t.Errorf("untracked.txt = %q, want unchanged %q", got, "keep me")
	}
}

func TestFR_BR_02_ResetWhileDetachedMovesHEADNotTheCheckedOutBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	thirdSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	if err := c.CheckoutCommit(context.Background(), wt, thirdSHA); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Reset(context.Background(), wt, firstSHA, core.ResetHard); err != nil {
		t.Fatal(err)
	}

	if branch := currentBranch(t, repo.Dir); branch != "" {
		t.Errorf("current branch = %s, want still detached", branch)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != firstSHA {
		t.Errorf("HEAD = %s, want %s", got, firstSHA)
	}
	if got := branchTip(t, repo.Dir, "master"); got != thirdSHA {
		t.Errorf("master = %s, want unchanged %s", got, thirdSHA)
	}
}

func TestFR_BR_03_UncommittedCountCountsTrackedChangesNotUntrackedFiles(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "third")

	// a.txt: unstaged modification.
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("a2"), 0o644); err != nil {
		t.Fatal(err)
	}
	// b.txt: staged, then further modified — one record, not two.
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b3"), 0o644); err != nil {
		t.Fatal(err)
	}
	// c.txt: staged deletion of a file still present on disk.
	gittest.Run(t, repo.Dir, "rm", "--cached", "--", "c.txt")
	// Two untracked files, excluded from the count.
	if err := os.WriteFile(filepath.Join(repo.Dir, "d.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "e.txt"), []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, wt := openWorktree(t, repo.Dir)

	got, err := c.UncommittedCount(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Errorf("UncommittedCount = %d, want 3", got)
	}
}

func TestFR_BR_03_UncommittedCountIsZeroOnACleanWorkingTree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c, wt := openWorktree(t, repo.Dir)

	got, err := c.UncommittedCount(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("UncommittedCount = %d, want 0", got)
	}
}

func TestFR_BR_04_ResetReturnsThePreviousHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "seed.txt", "c", "third")
	thirdSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	result, err := c.Reset(context.Background(), wt, firstSHA, core.ResetHard)
	if err != nil {
		t.Fatal(err)
	}

	if result.PreviousHead != thirdSHA {
		t.Errorf("PreviousHead = %s, want %s", result.PreviousHead, thirdSHA)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != firstSHA {
		t.Errorf("HEAD = %s, want %s", got, firstSHA)
	}
}

func TestFR_BR_02_ResetSoftRefusesDuringAnUnresolvedConflict(t *testing.T) {
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(context.Background(), wt, baseSHA, core.ResetSoft)

	coretest.AssertCode(t, err, core.GitFailed)
	if !mergeInProgress(repo.Dir) {
		t.Error("MERGE_HEAD gone, want still present (--soft refuses mid-merge)")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_02_ResetHardSucceedsDuringAnUnresolvedConflictEndingIt(t *testing.T) {
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(context.Background(), wt, baseSHA, core.ResetHard); err != nil {
		t.Fatal(err)
	}

	if mergeInProgress(repo.Dir) {
		t.Error("MERGE_HEAD still present, want gone (--hard ends an in-progress merge)")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != baseSHA {
		t.Errorf("HEAD = %s, want %s", got, baseSHA)
	}
	if got := readFile(t, repo.Dir, "conflict.txt"); got != "base" {
		t.Errorf("conflict.txt = %q, want baseSHA's own content %q", got, "base")
	}
}

func TestFR_BR_07_ResetKeepMatchesTargetExactlyOnACleanTree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "extra.txt", "x", "second")
	gittest.Commit(t, repo.Dir, "seed.txt", "c", "third")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(context.Background(), wt, firstSHA, core.ResetKeep); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "master"); got != firstSHA {
		t.Errorf("master = %s, want %s", got, firstSHA)
	}
	if got := readFile(t, repo.Dir, "seed.txt"); got != "a" {
		t.Errorf("seed.txt = %q, want firstSHA's own content %q", got, "a")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "extra.txt")); !os.IsNotExist(err) {
		t.Errorf("extra.txt = %v, want removed (absent from firstSHA's own tree)", err)
	}
}

func TestFR_BR_07_ResetKeepRefusesALocalChangeLeavingHEADIndexAndWorkingTreeUntouched(t *testing.T) {
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")
	beforeIndex := indexBlob(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(context.Background(), wt, firstSHA, core.ResetKeep)

	coretest.AssertCode(t, err, core.GitFailed)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
	if got := readFile(t, repo.Dir, "seed.txt"); got != beforeWorking {
		t.Errorf("working tree seed.txt = %q, want unchanged %q", got, beforeWorking)
	}
	if got := indexBlob(t, repo.Dir, "seed.txt"); got != beforeIndex {
		t.Errorf("index seed.txt = %q, want unchanged %q", got, beforeIndex)
	}
}

func TestFR_BR_07_ResetKeepRefusesDuringAnUnresolvedConflict(t *testing.T) {
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(context.Background(), wt, baseSHA, core.ResetKeep)

	coretest.AssertCode(t, err, core.GitFailed)
	if !mergeInProgress(repo.Dir) {
		t.Error("MERGE_HEAD gone, want still present (--keep refuses mid-merge)")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_02_ResetRefusesANonexistentSHAAsGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(context.Background(), wt, "0000000000000000000000000000000000000000", core.ResetHard)

	coretest.AssertCode(t, err, core.GitFailed)
}

// mergeConflictFixture builds a merge conflict in dir left unresolved
// (MERGE_HEAD set), returning the two branches' common ancestor.
func mergeConflictFixture(t *testing.T, dir string) (baseSHA string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "base", "base")
	baseSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "branch", "other")
	gittest.Commit(t, dir, "conflict.txt", "master-side", "master change")
	gittest.Run(t, dir, "checkout", "other")
	gittest.Commit(t, dir, "conflict.txt", "other-side", "other change")
	gittest.Run(t, dir, "checkout", "master")
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"merge", "other"}}) // conflict expected
	return baseSHA
}

func mergeInProgress(dir string) bool {
	return operation(filepath.Join(dir, ".git")) == "merge"
}

// resetFixture builds three commits and a dirty working tree: firstSHA's
// tree has only seed.txt; the second commit adds extra.txt; the third edits
// seed.txt, which then gets a staged change and an unstaged one on top.
func resetFixture(t *testing.T, dir string) (firstSHA string) {
	t.Helper()
	gittest.Commit(t, dir, "seed.txt", "a", "first")
	firstSHA = gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "extra.txt", "x", "second")
	gittest.Commit(t, dir, "seed.txt", "c", "third")

	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("staged"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "--", "seed.txt")
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("staged-then-unstaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	return firstSHA
}

// readFile returns name's working-tree content in dir.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// indexBlob returns name's staged content in dir.
func indexBlob(t *testing.T, dir, name string) string {
	t.Helper()
	return gittest.Out(t, dir, "show", ":"+name)
}

// trackedInIndex reports whether name is in dir's index right now.
func trackedInIndex(t *testing.T, dir, name string) bool {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"ls-files", "--error-unmatch", "--", name},
	})
	if err == nil {
		return true
	}
	if result.ExitCode == 1 {
		return false
	}
	t.Fatalf("ls-files --error-unmatch %s: %v", name, err)
	return false
}

// conflictingCheckoutFixture builds an older commit (returned) whose
// conflict.txt differs from an uncommitted change made on top of a newer
// commit, so any switch to the older commit is refused as "would be
// overwritten" (exit 1).
func conflictingCheckoutFixture(t *testing.T, dir string) (olderSHA string) {
	t.Helper()
	gittest.Commit(t, dir, "conflict.txt", "old", "older")
	olderSHA = gittest.HeadSHA(t, dir)
	gittest.Commit(t, dir, "conflict.txt", "new", "newer")
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	return olderSHA
}

// branchExists reports whether name is a local branch in dir.
func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"show-ref", "--verify", "--quiet", "refs/heads/" + name},
	})
	if err == nil {
		return true
	}
	if result.ExitCode == 1 {
		return false
	}
	t.Fatalf("show-ref refs/heads/%s: %v", name, err)
	return false
}

// branchTip returns local branch name's commit sha in dir.
func branchTip(t *testing.T, dir, name string) string {
	t.Helper()
	return gittest.Out(t, dir, "rev-parse", "refs/heads/"+name)
}

// currentBranch returns dir's current branch name, "" if detached.
func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"symbolic-ref", "--short", "-q", "HEAD"},
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(result.Stdout))
}
