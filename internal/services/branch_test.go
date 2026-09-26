package services

import (
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
	t.Parallel()
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
			coretest.AssertCode(t, remapDirty(tt.err, git.Result{ExitCode: tt.exit}), tt.want)
		})
	}
}

func TestFR_BR_02_ResetFlagMapsModeToGitFlag(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	featureTip := branchTip(t, repo.Dir, "feature")
	mainTip := branchTip(t, repo.Dir, "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.RenameBranch(t.Context(), wt, "feature", "renamed"); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	featureTip := branchTip(t, repo.Dir, "feature")

	c, wt := openWorktree(t, repo.Dir)

	err := c.RenameBranch(t.Context(), wt, "feature", "-badname")

	coretest.AssertCode(t, err, core.InvalidName)
	if got := branchTip(t, repo.Dir, "feature"); got != featureTip {
		t.Errorf("feature = %s, want unchanged %s (unrenamed)", got, featureTip)
	}
}

func TestFR_BR_06_RenameBranchRejectsADuplicateNewNameAsGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "branch", "taken")
	featureTip := branchTip(t, repo.Dir, "feature")
	takenTip := branchTip(t, repo.Dir, "taken")

	c, wt := openWorktree(t, repo.Dir)

	err := c.RenameBranch(t.Context(), wt, "feature", "taken")

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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "merged")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DeleteBranch(t.Context(), wt, "merged", false); err != nil {
		t.Fatal(err)
	}

	if branchExists(t, repo.Dir, "merged") {
		t.Error("merged still exists, want deleted")
	}
}

func TestFR_BR_06_DeleteBranchRefusesAnUnmergedBranchAsCodeUnmerged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "topic")
	gittest.Run(t, repo.Dir, "checkout", "topic")
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	err := c.DeleteBranch(t.Context(), wt, "topic", false)

	coretest.AssertCode(t, err, core.Unmerged)
	if !branchExists(t, repo.Dir, "topic") {
		t.Error("topic gone, want still present (unforced delete refuses)")
	}
}

func TestFR_BR_06_DeleteBranchForcedDeletesAnUnmergedBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "topic")
	gittest.Run(t, repo.Dir, "checkout", "topic")
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.DeleteBranch(t.Context(), wt, "topic", true); err != nil {
		t.Fatal(err)
	}

	if branchExists(t, repo.Dir, "topic") {
		t.Error("topic still exists, want deleted")
	}
}

func TestFR_BR_06_DeleteBranchRefusesABranchCheckedOutInAnotherWorktreeAsGitFailedNotUnmerged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "other")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "other")

	c, wt := openWorktree(t, repo.Dir)

	err := c.DeleteBranch(t.Context(), wt, "other", false)
	coretest.AssertCode(t, err, core.GitFailed)

	err = c.DeleteBranch(t.Context(), wt, "other", true)
	coretest.AssertCode(t, err, core.GitFailed)

	if !branchExists(t, repo.Dir, "other") {
		t.Error("other gone, want still present")
	}
	if got := currentBranch(t, linked); got != "other" {
		t.Errorf("linked worktree's branch = %s, want still other", got)
	}
}

func TestFR_BR_07_ResetFlagMapsKeepToGitFlag(t *testing.T) {
	t.Parallel()
	got, err := resetFlag(core.ResetKeep)
	if err != nil {
		t.Fatalf("resetFlag(keep) error = %v, want nil", err)
	}
	if got != "--keep" {
		t.Errorf("resetFlag(keep) = %q, want \"--keep\"", got)
	}
}

func TestFR_BR_01_CreateBranchWithoutCheckoutLeavesCurrentBranchAlone(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", firstSHA, false); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", firstSHA, true); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	originalTip := branchTip(t, repo.Dir, "feature")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(t.Context(), wt, "feature", sha, false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "already exists") {
		t.Errorf("GitStderr = %q, want it to say the branch already exists", ce.GitStderr)
	}
	if got := branchTip(t, repo.Dir, "feature"); got != originalTip {
		t.Errorf("feature = %s, want unchanged %s", got, originalTip)
	}
}

func TestFR_BR_01_CreateBranchRejectsAnInvalidNameBeforeTouchingGit(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(t.Context(), wt, "-bad", sha, false)

	ce := coretest.AsError(t, err, core.InvalidName)
	if ce.Key != "invalidBranchName" || ce.Args["name"] != "-bad" {
		t.Errorf("Key/Args = %q/%+v, want invalidBranchName/name=-bad", ce.Key, ce.Args)
	}
	if branchExists(t, repo.Dir, "-bad") {
		t.Error("branch -bad was created, want none")
	}
}

func TestFR_BR_01_CreateBranchWithCheckoutIsAtomicOnADirtyConflict(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(t.Context(), wt, "old", olderSHA, true)

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if branchExists(t, repo.Dir, "old") {
		t.Error("branch old was created despite the atomic checkout failing, want none")
	}
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
	if got := readFile(t, repo.Dir, "conflict.txt"); got != "dirty" {
		t.Errorf("conflict.txt = %q, want the uncommitted content unchanged", got)
	}
}

func TestFR_BR_01_CreateBranchStartsAtTheFullRefNamedWhenATagAndABranchShareItsName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	branchSHA, tagSHA := branchAndTagNamedV1(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	for _, tc := range []struct {
		name, start, want string
		checkout          bool
	}{
		{"from-branch", "refs/heads/v1", branchSHA, false},
		{"from-tag", "refs/tags/v1", tagSHA, false},
		{"from-branch-switched", "refs/heads/v1", branchSHA, true},
		{"from-tag-switched", "refs/tags/v1", tagSHA, true},
	} {
		if err := c.CreateBranch(t.Context(), wt, tc.name, tc.start, tc.checkout); err != nil {
			t.Fatalf("CreateBranch(%s, %s, checkout %v): %v", tc.name, tc.start, tc.checkout, err)
		}
		if got := branchTip(t, repo.Dir, tc.name); got != tc.want {
			t.Errorf("%s = %s, want %s's commit %s", tc.name, got, tc.start, tc.want)
		}
	}
}

func TestFR_BR_01_CreateBranchFromARemoteBranchsFullRefTracksIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	withOrigin(t, repo.Dir, "feature")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", "refs/remotes/origin/feature", false); err != nil {
		t.Fatal(err)
	}

	if got := upstreamOf(t, repo.Dir, "feature"); got != "origin/feature" {
		t.Errorf("upstream = %q, want origin/feature", got)
	}
}

func TestFR_BR_05_CheckoutSwitchesToAnExistingLocalBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Checkout(t.Context(), wt, "feature"); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	withOrigin(t, repo.Dir, "shared")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Checkout(t.Context(), wt, "shared"); err != nil {
		t.Fatal(err)
	}

	if got := currentBranch(t, repo.Dir); got != "shared" {
		t.Errorf("current branch = %s, want shared", got)
	}
	if got := upstreamOf(t, repo.Dir, "shared"); got != "origin/shared" {
		t.Errorf("upstream = %q, want origin/shared", got)
	}
}

func TestFR_BR_09_CheckoutOfARemoteBranchsFullRefTracksItDespiteATagOfItsName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	withOrigin(t, repo.Dir, "shared")
	gittest.Run(t, repo.Dir, "tag", "shared")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.Checkout(t.Context(), wt, "refs/remotes/origin/shared"); err != nil {
		t.Fatal(err)
	}

	if got := gittest.Out(t, repo.Dir, "symbolic-ref", "HEAD"); got != "refs/heads/shared" {
		t.Errorf("HEAD = %s, want refs/heads/shared", got) // the short form is ambiguous here
	}
	if got := upstreamOf(t, repo.Dir, "shared"); got != "origin/shared" {
		t.Errorf("upstream = %q, want origin/shared", got)
	}
}

func TestFR_BR_05_CheckoutRefusesADirtyConflictAsDirtyWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature", olderSHA) // a plain ref create, unaffected by the dirty working tree
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Checkout(t.Context(), wt, "feature")

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
}

func TestFR_BR_05_CheckoutNamesTheOtherWorktreeWhenABranchIsAlreadyCheckedOutThere(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	linkedPath := gitPath(t, linked)
	before := currentBranch(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.Checkout(t.Context(), wt, "feature")

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, linkedPath) {
		t.Errorf("GitStderr = %q, want it to name the other worktree's path %q", ce.GitStderr, linkedPath)
	}
	if got := currentBranch(t, repo.Dir); got != before {
		t.Errorf("current branch = %s, want unchanged %s", got, before)
	}
}

func TestFR_BR_08_CheckoutCommitDetachesHEADAtTheGivenSHA(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CheckoutCommit(t.Context(), wt, firstSHA); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CheckoutCommit(t.Context(), wt, olderSHA)

	coretest.AssertCode(t, err, core.DirtyWorktree)
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_08_CheckoutCommitSucceedsEvenWhenAlreadyDetachedInAnotherWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linked, sha)

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CheckoutCommit(t.Context(), wt, sha); err != nil {
		t.Errorf("CheckoutCommit = %v, want success (a detached commit may be shared across worktrees)", err)
	}
}

func TestQA_REL_02_BranchActionsFailCleanlyOnAnEmptyRepo(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t) // no commit: an unborn HEAD

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", "HEAD", false); err == nil {
		t.Error("CreateBranch on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}

	if err := c.Checkout(t.Context(), wt, "anything"); err == nil {
		t.Error("Checkout on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}

	if err := c.CheckoutCommit(t.Context(), wt, "HEAD"); err == nil {
		t.Error("CheckoutCommit on an empty repo = nil, want an error")
	} else {
		coretest.AssertCode(t, err, core.GitFailed)
	}
}

func TestFR_BR_02_ResetSoftMovesBranchLeavesIndexAndWorkingTreeUntouched(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")
	beforeIndex := indexBlob(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(t.Context(), wt, firstSHA, core.ResetSoft); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(t.Context(), wt, firstSHA, core.ResetMixed); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	writeFile(t, repo.Dir, "untracked.txt", "keep me")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(t.Context(), wt, firstSHA, core.ResetHard); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	thirdSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	if err := c.CheckoutCommit(t.Context(), wt, thirdSHA); err != nil {
		t.Fatal(err)
	}

	if _, err := c.Reset(t.Context(), wt, firstSHA, core.ResetHard); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "third")

	// a.txt: unstaged modification.
	writeFile(t, repo.Dir, "a.txt", "a2")
	// b.txt: staged, then further modified — one record, not two.
	writeFile(t, repo.Dir, "b.txt", "b2")
	gittest.Run(t, repo.Dir, "add", "--", "b.txt")
	writeFile(t, repo.Dir, "b.txt", "b3")
	// c.txt: staged deletion of a file still present on disk.
	gittest.Run(t, repo.Dir, "rm", "--cached", "--", "c.txt")
	// Two untracked files, excluded from the count.
	writeFile(t, repo.Dir, "d.txt", "d")
	writeFile(t, repo.Dir, "e.txt", "e")

	c, wt := openWorktree(t, repo.Dir)

	got, err := c.UncommittedCount(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Errorf("UncommittedCount = %d, want 3", got)
	}
}

func TestFR_BR_03_UncommittedCountIsZeroOnACleanWorkingTree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c, wt := openWorktree(t, repo.Dir)

	got, err := c.UncommittedCount(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("UncommittedCount = %d, want 0", got)
	}
}

func TestFR_BR_04_ResetReturnsThePreviousHEAD(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "seed.txt", "c", "third")
	thirdSHA := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	result, err := c.Reset(t.Context(), wt, firstSHA, core.ResetHard)
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
	t.Parallel()
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(t.Context(), wt, baseSHA, core.ResetSoft)

	coretest.AssertCode(t, err, core.GitFailed)
	if !mergeInProgress(repo.Dir) {
		t.Error("MERGE_HEAD gone, want still present (--soft refuses mid-merge)")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_02_ResetHardSucceedsDuringAnUnresolvedConflictEndingIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(t.Context(), wt, baseSHA, core.ResetHard); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "extra.txt", "x", "second")
	gittest.Commit(t, repo.Dir, "seed.txt", "c", "third")

	c, wt := openWorktree(t, repo.Dir)

	if _, err := c.Reset(t.Context(), wt, firstSHA, core.ResetKeep); err != nil {
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
	t.Parallel()
	repo := gittest.New(t)
	firstSHA := resetFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)
	beforeWorking := readFile(t, repo.Dir, "seed.txt")
	beforeIndex := indexBlob(t, repo.Dir, "seed.txt")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(t.Context(), wt, firstSHA, core.ResetKeep)

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
	t.Parallel()
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(t.Context(), wt, baseSHA, core.ResetKeep)

	coretest.AssertCode(t, err, core.GitFailed)
	if !mergeInProgress(repo.Dir) {
		t.Error("MERGE_HEAD gone, want still present (--keep refuses mid-merge)")
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
}

func TestFR_BR_02_ResetRefusesANonexistentSHAAsGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(t.Context(), wt, "0000000000000000000000000000000000000000", core.ResetHard)

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
	_, _ = (git.Runner{}).Run(t.Context(), git.Call{Dir: dir, Args: []string{"merge", "other"}}) // conflict expected
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

	writeFile(t, dir, "seed.txt", "staged")
	gittest.Run(t, dir, "add", "--", "seed.txt")
	writeFile(t, dir, "seed.txt", "staged-then-unstaged")
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
	result, err := (git.Runner{}).Run(t.Context(), git.Call{
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
	writeFile(t, dir, "conflict.txt", "dirty")
	return olderSHA
}

// branchExists reports whether name is a local branch in dir.
func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	result, err := (git.Runner{}).Run(t.Context(), git.Call{
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
	result, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"symbolic-ref", "--short", "-q", "HEAD"},
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(result.Stdout))
}

// branchAndTagNamedV1 makes a branch v1 at the first commit and an
// annotated tag v1 at the second, then commits past both: the bare name v1
// is ambiguous.
func branchAndTagNamedV1(t *testing.T, dir string) (branchSHA, tagSHA string) {
	t.Helper()
	gittest.Commit(t, dir, "seed.txt", "a", "first")
	branchSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "branch", "v1")
	gittest.Commit(t, dir, "seed.txt", "b", "second")
	tagSHA = gittest.HeadSHA(t, dir)
	gittest.Run(t, dir, "tag", "-a", "-m", "release", "v1")
	gittest.Commit(t, dir, "seed.txt", "c", "third")
	return branchSHA, tagSHA
}

// withOrigin adds a bare remote origin holding dir's master as branch, and
// fetches it.
func withOrigin(t *testing.T, dir, branch string) {
	t.Helper()
	bareDir := t.TempDir()
	gittest.Run(t, bareDir, "init", "--bare")
	gittest.Run(t, dir, "remote", "add", "origin", bareDir)
	gittest.Run(t, dir, "push", "origin", "master:"+branch)
	gittest.Run(t, dir, "fetch", "origin")
}

// upstreamOf returns local branch name's upstream in short form.
func upstreamOf(t *testing.T, dir, name string) string {
	t.Helper()
	return gittest.Out(t, dir, "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+name)
}
