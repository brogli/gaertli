// Requirements: FR-BR-01, FR-BR-02, FR-BR-03, FR-BR-04, FR-BR-05, FR-BR-06, FR-BR-08, FR-BR-09, FR-BR-10, FR-BR-14, QA-REL-02

package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestRemapDirtyClassifiesByExitCodeNotText(t *testing.T) {
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

func TestRenameBranchRefusesBadNewNamesThenRenamesANonCurrentBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "branch", "taken")
	featureTip := branchTip(t, repo.Dir, "feature")
	takenTip := branchTip(t, repo.Dir, "taken")
	mainTip := branchTip(t, repo.Dir, "master")

	c, wt := openWorktree(t, repo.Dir)

	coretest.AssertCode(t, c.RenameBranch(t.Context(), wt, "feature", "-badname"), core.InvalidName)

	ce := coretest.AsError(t, c.RenameBranch(t.Context(), wt, "feature", "taken"), core.GitFailed)
	if ce.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
	if got := branchTip(t, repo.Dir, "feature"); got != featureTip {
		t.Errorf("feature = %s, want unchanged %s after the refusals", got, featureTip)
	}
	if got := branchTip(t, repo.Dir, "taken"); got != takenTip {
		t.Errorf("taken = %s, want unchanged %s", got, takenTip)
	}

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

// git renames a branch checked out elsewhere and moves that worktree's HEAD
// along; only a branch being rebased or bisected is refused up front.
func TestRenameBranchRenamesABranchCheckedOutInAnotherWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "other")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "other")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.RenameBranch(t.Context(), wt, "other", "renamed"); err != nil {
		t.Fatal(err)
	}

	if !branchExists(t, repo.Dir, "renamed") {
		t.Error("renamed missing, want it created")
	}
	if branchExists(t, repo.Dir, "other") {
		t.Error("other still exists, want renamed away")
	}
	if got := currentBranch(t, linked); got != "renamed" {
		t.Errorf("linked worktree's branch = %s, want renamed (git moves its HEAD)", got)
	}
}

func TestRenameAndDeleteBranchRefuseABranchBeingRebasedElsewhere(t *testing.T) {
	t.Parallel()
	repo, wtDir := rebaseInProgressFixture(t)
	wtPath := gitPath(t, wtDir)
	beforeSHA := branchTip(t, repo.Dir, "B")

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	for name, err := range map[string]error{
		"rename": c.RenameBranch(t.Context(), wt, "B", "B2"),
		"delete": c.DeleteBranch(t.Context(), wt, "B", true),
	} {
		ce := coretest.AsError(t, err, core.GitFailed)
		if ce.Key != "branchRebasingElsewhere" {
			t.Errorf("%s: Key = %q, want branchRebasingElsewhere", name, ce.Key)
		}
		if ce.Args["branch"] != "B" || ce.Args["path"] != wtPath {
			t.Errorf("%s: Args = %+v, want branch=B path=%s", name, ce.Args, wtPath)
		}
	}
	for _, e := range rec.snapshot() {
		if strings.HasPrefix(e.Command, "git branch ") {
			t.Errorf("recorded %q, want no branch call before the refusal", e.Command)
		}
	}
	if got := branchTip(t, repo.Dir, "B"); got != beforeSHA {
		t.Errorf("B = %s, want unchanged %s", got, beforeSHA)
	}
	if branchExists(t, repo.Dir, "B2") {
		t.Error("B2 exists, want the rename refused")
	}
}

// TestRenameAndDeleteBranchRefuseABranchBeingBisected asks the
// bisecting worktree itself to rename (refuseBranchInUse finds a branch in
// use in any worktree, this one included) and the main worktree to delete.
func TestRenameAndDeleteBranchRefuseABranchBeingBisected(t *testing.T) {
	t.Parallel()
	repo, wtDir := bisectInProgressFixture(t)
	wtPath := gitPath(t, wtDir)

	cWT, bisecting := openWorktree(t, wtDir)
	cMain, main := openWorktree(t, repo.Dir)
	recWT := recordCommands(t, cWT, bisecting)
	recMain := recordCommands(t, cMain, main)

	for name, err := range map[string]error{
		"rename": cWT.RenameBranch(t.Context(), bisecting, "B", "B2"),
		"delete": cMain.DeleteBranch(t.Context(), main, "B", true),
	} {
		ce := coretest.AsError(t, err, core.GitFailed)
		if ce.Key != "branchBisectingElsewhere" {
			t.Errorf("%s: Key = %q, want branchBisectingElsewhere", name, ce.Key)
		}
		if ce.Args["branch"] != "B" || ce.Args["path"] != wtPath {
			t.Errorf("%s: Args = %+v, want branch=B path=%s", name, ce.Args, wtPath)
		}
	}
	for _, rec := range []*commandRecorder{recWT, recMain} {
		for _, e := range rec.snapshot() {
			if strings.HasPrefix(e.Command, "git branch ") {
				t.Errorf("recorded %q, want no branch call before the refusal", e.Command)
			}
		}
	}
	if !branchExists(t, wtDir, "B") {
		t.Error("B gone, want still present")
	}
	if branchExists(t, wtDir, "B2") {
		t.Error("B2 exists, want the rename refused")
	}
}

func TestDeleteBranchRefusesAnUnmergedBranchUnlessForcedThenDeletesAMergedOne(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "merged")
	gittest.Run(t, repo.Dir, "branch", "topic")
	gittest.Run(t, repo.Dir, "checkout", "topic")
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)

	coretest.AssertCode(t, c.DeleteBranch(t.Context(), wt, "topic", false), core.Unmerged)
	if !branchExists(t, repo.Dir, "topic") {
		t.Error("topic gone, want still present (unforced delete refuses)")
	}

	if err := c.DeleteBranch(t.Context(), wt, "merged", false); err != nil {
		t.Fatal(err)
	}
	if branchExists(t, repo.Dir, "merged") {
		t.Error("merged still exists, want deleted")
	}

	if err := c.DeleteBranch(t.Context(), wt, "topic", true); err != nil {
		t.Fatal(err)
	}
	if branchExists(t, repo.Dir, "topic") {
		t.Error("topic still exists, want deleted")
	}
}

func TestDeleteBranchRefusesABranchCheckedOutInAnotherWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "other")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "other")
	linkedPath := gitPath(t, linked)

	c, wt := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, wt)

	for _, force := range []bool{false, true} {
		err := c.DeleteBranch(t.Context(), wt, "other", force)

		ce := coretest.AsError(t, err, core.GitFailed)
		if ce.Key != "branchCheckedOutSwitchFirst" {
			t.Errorf("force=%v: Key = %q, want branchCheckedOutSwitchFirst", force, ce.Key)
		}
		if ce.Args["branch"] != "other" || ce.Args["path"] != linkedPath {
			t.Errorf("force=%v: Args = %+v, want branch=other path=%s", force, ce.Args, linkedPath)
		}
	}
	for _, e := range rec.snapshot() {
		if strings.HasPrefix(e.Command, "git branch ") {
			t.Errorf("recorded %q, want no branch call before the refusal", e.Command)
		}
	}
	if !branchExists(t, repo.Dir, "other") {
		t.Error("other gone, want still present")
	}
	if got := currentBranch(t, linked); got != "other" {
		t.Errorf("linked worktree's branch = %s, want still other", got)
	}
}

// TestDeleteBranchSucceedsOnTheBareMainReposHEADBranch covers a
// bare main repo: its HEAD points to a branch, but a bare repo has no
// checkout, so that branch is not "checked out" anywhere and delete must
// not refuse it the way it would a branch actually checked out in another
// worktree.
func TestDeleteBranchSucceedsOnTheBareMainReposHEADBranch(t *testing.T) {
	t.Parallel()
	bareDir, linkedDir, head := bareRepoWithWorktree(t)

	c, wt := openWorktree(t, linkedDir)

	if err := c.DeleteBranch(t.Context(), wt, head, true); err != nil {
		t.Fatal(err)
	}

	if branchExists(t, bareDir, head) {
		t.Errorf("%s still exists, want deleted", head)
	}
}

func TestCreateBranchWithoutCheckoutLeavesCurrentBranchAlone(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", string(firstSHA), false); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "feature"); got != firstSHA {
		t.Errorf("feature = %s, want %s", got, firstSHA)
	}
	if got := currentBranch(t, repo.Dir); got != "master" {
		t.Errorf("current branch = %s, want master (unchanged)", got)
	}
}

func TestCreateBranchWithCheckoutSwitchesToIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CreateBranch(t.Context(), wt, "feature", string(firstSHA), true); err != nil {
		t.Fatal(err)
	}

	if got := branchTip(t, repo.Dir, "feature"); got != firstSHA {
		t.Errorf("feature = %s, want %s", got, firstSHA)
	}
	if got := currentBranch(t, repo.Dir); got != "feature" {
		t.Errorf("current branch = %s, want feature", got)
	}
}

func TestCreateBranchRejectsADuplicateNameAsGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	originalTip := branchTip(t, repo.Dir, "feature")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(t.Context(), wt, "feature", string(sha), false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
	if got := branchTip(t, repo.Dir, "feature"); got != originalTip {
		t.Errorf("feature = %s, want unchanged %s", got, originalTip)
	}
}

func TestCreateBranchRejectsAnInvalidNameBeforeTouchingGit(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	err := c.CreateBranch(t.Context(), wt, "-bad", string(sha), false)

	ce := coretest.AsError(t, err, core.InvalidName)
	if ce.Key != "invalidBranchName" || ce.Args["name"] != "-bad" {
		t.Errorf("Key/Args = %q/%+v, want invalidBranchName/name=-bad", ce.Key, ce.Args)
	}
	if branchExists(t, repo.Dir, "-bad") {
		t.Error("branch -bad was created, want none")
	}
}

func TestCheckoutRefusesADirtyConflictAsDirtyWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	olderSHA := conflictingCheckoutFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature", string(olderSHA)) // a plain ref create, unaffected by the dirty working tree
	beforeBranch := currentBranch(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	for name, err := range map[string]error{
		"Checkout":                   c.Checkout(t.Context(), wt, "feature"),
		"CheckoutCommit":             c.CheckoutCommit(t.Context(), wt, olderSHA),
		"CreateBranch with checkout": c.CreateBranch(t.Context(), wt, "old", string(olderSHA), true),
	} {
		t.Run(name, func(t *testing.T) { coretest.AssertCode(t, err, core.DirtyWorktree) })
	}

	if branchExists(t, repo.Dir, "old") {
		t.Error("branch old was created despite the atomic checkout failing, want none")
	}
	if got := currentBranch(t, repo.Dir); got != beforeBranch {
		t.Errorf("current branch = %s, want unchanged %s", got, beforeBranch)
	}
	if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
		t.Errorf("HEAD = %s, want unchanged %s", got, beforeHEAD)
	}
	if got := readFile(t, repo.Dir, "conflict.txt"); got != "dirty" {
		t.Errorf("conflict.txt = %q, want the uncommitted content unchanged", got)
	}
}

func TestCreateBranchStartsAtTheFullRefNamedWhenATagAndABranchShareItsName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	branchSHA, tagSHA := branchAndTagNamedV1(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	for _, tc := range []struct {
		name, start string
		want        core.SHA
		checkout    bool
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

func TestCreateBranchFromARemoteBranchSetsNoUpstream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, remoteBranch, newName string }{
		{"a different name", "master", "topic"},
		{"the remote branch's own name", "feature", "feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
			withOrigin(t, repo.Dir, tc.remoteBranch)
			originTip := gittest.Out(t, repo.Dir, "rev-parse", "refs/remotes/origin/"+tc.remoteBranch)

			c, wt := openWorktree(t, repo.Dir)

			if err := c.CreateBranch(t.Context(), wt, tc.newName, "refs/remotes/origin/"+tc.remoteBranch, false); err != nil {
				t.Fatal(err)
			}

			if got := branchTip(t, repo.Dir, tc.newName); got != core.SHA(originTip) {
				t.Errorf("%s = %s, want the remote branch's commit %s", tc.newName, got, originTip)
			}
			if got := upstreamOf(t, repo.Dir, tc.newName); got != "" {
				t.Errorf("upstream = %q, want none", got)
			}
		})
	}
}

// A branch started at a remote branch tracks nothing until its first push
// gives it its own upstream.
func TestABranchCreatedFromARemoteBranchGetsItsOwnUpstreamOnFirstPush(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	origin := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "origin", "master")
	gittest.Run(t, repo.Dir, "fetch", "origin")
	localTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	if err := c.CreateBranch(t.Context(), wt, "topic", "refs/remotes/origin/master", true); err != nil {
		t.Fatal(err)
	}

	target, err := c.PushTarget(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if want := (core.PushTarget{Remote: "origin", Branch: "topic", HasUpstream: false}); target != want {
		t.Errorf("PushTarget = %+v, want %+v", target, want)
	}

	waitForJobState(t, c, wt, jobID(t)(c.Push(t.Context(), wt, "origin", "topic")), core.JobDone)

	if got := gittest.Out(t, origin, "rev-parse", "topic"); core.SHA(got) != localTip {
		t.Errorf("origin's topic = %q, want %q", got, localTip)
	}
	if got := upstreamOf(t, repo.Dir, "topic"); got != "origin/topic" {
		t.Errorf("upstream = %q, want origin/topic", got)
	}
}

func TestCheckoutSwitchesToAnExistingLocalBranch(t *testing.T) {
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

func TestCheckoutDWIMsARemoteTrackingBranchIntoALocalOne(t *testing.T) {
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

func TestCheckoutOfARemoteBranchsFullRefTracksItDespiteATagOfItsName(t *testing.T) {
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

func TestCheckoutNamesTheOtherWorktreeWhenABranchIsAlreadyCheckedOutThere(t *testing.T) {
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

func TestCheckoutCommitDetachesHEADAtTheGivenSHA(t *testing.T) {
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

func TestCheckoutCommitSucceedsEvenWhenAlreadyDetachedInAnotherWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linked, string(sha))

	c, wt := openWorktree(t, repo.Dir)

	if err := c.CheckoutCommit(t.Context(), wt, sha); err != nil {
		t.Errorf("CheckoutCommit = %v, want success (a detached commit may be shared across worktrees)", err)
	}
}

func TestBranchActionsFailCleanlyOnAnEmptyRepo(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t) // no commit: an unborn HEAD

	c, wt := openWorktree(t, repo.Dir)

	for name, err := range map[string]error{
		"CreateBranch":   c.CreateBranch(t.Context(), wt, "feature", "HEAD", false),
		"Checkout":       c.Checkout(t.Context(), wt, "anything"),
		"CheckoutCommit": c.CheckoutCommit(t.Context(), wt, "HEAD"),
	} {
		t.Run(name, func(t *testing.T) { coretest.AssertCode(t, err, core.GitFailed) })
	}
}

func TestResetSoftMovesBranchLeavesIndexAndWorkingTreeUntouched(t *testing.T) {
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

func TestResetMixedLeavesWorkingTreeUntracksPathsAbsentFromTarget(t *testing.T) {
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

func TestResetHardMatchesTargetExactlyLeavesUntrackedFileAlone(t *testing.T) {
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

func TestResetWhileDetachedMovesHEADNotTheCheckedOutBranch(t *testing.T) {
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

func TestUncommittedCountCountsTrackedChangesNotUntrackedFiles(t *testing.T) {
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

func TestUncommittedCountIsZeroOnACleanWorkingTree(t *testing.T) {
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

func TestResetReturnsThePreviousHEAD(t *testing.T) {
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

func TestResetSoftAndKeepRefuseDuringAnUnresolvedConflictAndHardEndsIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	baseSHA := mergeConflictFixture(t, repo.Dir)
	beforeHEAD := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	for _, mode := range []core.ResetMode{core.ResetSoft, core.ResetKeep} {
		_, err := c.Reset(t.Context(), wt, baseSHA, mode)

		coretest.AssertCode(t, err, core.GitFailed)
		if !mergeInProgress(repo.Dir) {
			t.Errorf("MERGE_HEAD gone, want still present (%s refuses mid-merge)", mode)
		}
		if got := gittest.HeadSHA(t, repo.Dir); got != beforeHEAD {
			t.Errorf("HEAD = %s after %s, want unchanged %s", got, mode, beforeHEAD)
		}
	}

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

func TestResetKeepMatchesTargetExactlyOnACleanTree(t *testing.T) {
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

func TestResetKeepRefusesALocalChangeLeavingHEADIndexAndWorkingTreeUntouched(t *testing.T) {
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

func TestResetRefusesANonexistentSHAAsGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c, wt := openWorktree(t, repo.Dir)

	_, err := c.Reset(t.Context(), wt, "0000000000000000000000000000000000000000", core.ResetHard)

	coretest.AssertCode(t, err, core.GitFailed)

	_, err = resetFlag(core.ResetMode("bogus"))
	coretest.AssertCode(t, err, core.Internal)
}

// mergeConflictFixture builds a merge conflict in dir left unresolved
// (MERGE_HEAD set), returning the two branches' common ancestor.
func mergeConflictFixture(t *testing.T, dir string) (baseSHA core.SHA) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "base", Files: map[string]string{"conflict.txt": "base"}},
		gittest.Step{Branch: "other", From: "master", Message: "other change", Files: map[string]string{"conflict.txt": "other-side"}},
		gittest.Step{Message: "master change", Files: map[string]string{"conflict.txt": "master-side"}},
	)
	gitStops(t, dir, "merge", "other")
	return shas[0]
}

func mergeInProgress(dir string) bool {
	return operation(filepath.Join(dir, ".git")) == "merge"
}

// resetFixture builds three commits and a dirty working tree: firstSHA's
// tree has only seed.txt; the second commit adds extra.txt; the third edits
// seed.txt, which then gets a staged change and an unstaged one on top.
func resetFixture(t *testing.T, dir string) (firstSHA core.SHA) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "first", Files: map[string]string{"seed.txt": "a"}},
		gittest.Step{Message: "second", Files: map[string]string{"extra.txt": "x"}},
		gittest.Step{Message: "third", Files: map[string]string{"seed.txt": "c"}},
	)
	firstSHA = shas[0]

	writeFile(t, dir, "seed.txt", "staged")
	gittest.Run(t, dir, "add", "--", "seed.txt")
	writeFile(t, dir, "seed.txt", "staged-then-unstaged")
	return firstSHA
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
func conflictingCheckoutFixture(t *testing.T, dir string) (olderSHA core.SHA) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "older", Files: map[string]string{"conflict.txt": "old"}},
		gittest.Step{Message: "newer", Files: map[string]string{"conflict.txt": "new"}},
	)
	writeFile(t, dir, "conflict.txt", "dirty")
	return shas[0]
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
func branchTip(t *testing.T, dir, name string) core.SHA {
	t.Helper()
	return core.SHA(gittest.Out(t, dir, "rev-parse", "refs/heads/"+name))
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
func branchAndTagNamedV1(t *testing.T, dir string) (branchSHA, tagSHA core.SHA) {
	t.Helper()
	shas := gittest.History(t, dir,
		gittest.Step{Message: "first", Files: map[string]string{"seed.txt": "a"}},
		gittest.Step{Message: "second", Files: map[string]string{"seed.txt": "b"}},
		gittest.Step{Message: "third", Files: map[string]string{"seed.txt": "c"}},
	)
	gittest.Run(t, dir, "branch", "v1", string(shas[0]))
	gittest.Run(t, dir, "tag", "-a", "-m", "release", "v1", string(shas[1]))
	return shas[0], shas[1]
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
