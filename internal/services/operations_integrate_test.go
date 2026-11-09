// Requirements: FR-BR-11, FR-BR-12, FR-SYNC-09

package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
)

// edit is a file's content in one commit.
type edit struct{ path, content string }

// diverge makes a repo with base.txt committed on master, a branch side one commit past it with
// side, and master one commit past it with main; master is checked out.
func diverge(t *testing.T, base string, side, main edit) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	gittest.History(t, repo.Dir,
		gittest.Step{Message: "base", Files: map[string]string{"base.txt": base}},
		gittest.Step{Branch: "side", From: "master", Message: "side commit", Files: map[string]string{side.path: side.content}},
		gittest.Step{Message: "main commit", Files: map[string]string{main.path: main.content}},
	)
	return repo
}

// divergedFixture is a repo whose master and side touch different files.
func divergedFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	return diverge(t, "base\n", edit{"side.txt", "side\n"}, edit{"main.txt", "main\n"})
}

// divergedConflictRepo is a repo whose master and side change base.txt differently.
func divergedConflictRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	return diverge(t, "base\n", edit{"base.txt", "side\n"}, edit{"base.txt", "main\n"})
}

func TestMergeAndRebaseRefuseBeforeGitRunsWithAnOperationOrDetachedHead(t *testing.T) {
	t.Parallel()
	repo := divergedConflictRepo(t)
	c, wt := openWorktree(t, repo.Dir)
	calls := map[string]func() error{
		"merge": func() error {
			_, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
			return err
		},
		"rebase": func() error {
			_, err := c.Rebase(t.Context(), wt, "refs/heads/side", false)
			return err
		},
	}
	sideTip := gittest.Out(t, repo.Dir, "rev-parse", "side")
	coretest.AssertCode(t, c.CherryPick(t.Context(), wt, []core.SHA{core.SHA(sideTip)}, core.PickOptions{}), core.Conflicted)
	rec := recordCommands(t, c, wt)

	for name, call := range calls {
		ce := coretest.AsError(t, call(), core.GitFailed)
		if ce.Key != "operationInProgress" || ce.Args["operation"] != "cherry-pick" {
			t.Errorf("%s: error = %+v, want key operationInProgress, operation cherry-pick", name, ce)
		}
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("git calls = %d, want 0 while an operation is in progress", got)
	}

	gittest.Run(t, repo.Dir, "cherry-pick", "--abort")
	gittest.Run(t, repo.Dir, "checkout", "--detach")
	head := gittest.HeadSHA(t, repo.Dir)
	for name, call := range calls {
		if ce := coretest.AsError(t, call(), core.GitFailed); ce.Key != "detachedHead" {
			t.Errorf("%s: Key = %q, want detachedHead", name, ce.Key)
		}
	}
	if after := gittest.HeadSHA(t, repo.Dir); after != head {
		t.Errorf("HEAD = %q, want unchanged %q", after, head)
	}
}

func TestMergeFastForwards(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "base.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "checkout", "-b", "side")
	gittest.Commit(t, repo.Dir, "side.txt", "side\n", "side commit")
	sideTip := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "checkout", "master")

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != core.IntegrateFastForward {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateFastForward)
	}
	if head := gittest.HeadSHA(t, repo.Dir); head != sideTip {
		t.Errorf("HEAD = %q, want side's tip %q", head, sideTip)
	}
}

func TestMergeMakesAMergeCommitRunsHooksAndIgnoresHostileConfig(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "config", "merge.ff", "only")
	gittest.Run(t, repo.Dir, "config", "branch.master.mergeOptions", "--squash")
	gittest.Run(t, repo.Dir, "config", "merge.autoStash", "true")
	gittest.Run(t, repo.Dir, "config", "pull.twohead", "ours")
	writeHook(t, repo.Dir, "post-merge", "echo ran > hook-ran.txt")
	mainTip := gittest.HeadSHA(t, repo.Dir)
	sideTip := gittest.Out(t, repo.Dir, "rev-parse", "side")

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != core.IntegrateMerged {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateMerged)
	}
	if parents := gittest.Out(t, repo.Dir, "rev-list", "--parents", "-n1", "HEAD"); !strings.HasSuffix(parents, string(mainTip)+" "+sideTip) {
		t.Errorf("parents line = %q, want parents %s %s", parents, mainTip, sideTip)
	}
	if subject := gittest.Out(t, repo.Dir, "log", "-1", "--format=%s"); subject != "Merge branch 'side'" {
		t.Errorf("subject = %q, want %q", subject, "Merge branch 'side'")
	}
	if !fileExists(filepath.Join(repo.Dir, "hook-ran.txt")) {
		t.Error("post-merge hook did not run")
	}
	if content := readFile(t, repo.Dir, "side.txt"); content != "side\n" {
		t.Errorf("side.txt = %q, want the side's change", content)
	}
}

func TestMergeNamesAFullRefWhenTheShortNameIsAmbiguous(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "branch", "x", "side")
	gittest.Run(t, repo.Dir, "tag", "x", "master")
	branchTip := gittest.Out(t, repo.Dir, "rev-parse", "refs/heads/x")

	c, wt := openWorktree(t, repo.Dir)
	if _, err := c.Merge(t.Context(), wt, "refs/heads/x", false); err != nil {
		t.Fatal(err)
	}
	if second := gittest.Out(t, repo.Dir, "rev-parse", "HEAD^2"); second != branchTip {
		t.Errorf("HEAD^2 = %q, want the branch tip %q", second, branchTip)
	}
	if subject := gittest.Out(t, repo.Dir, "log", "-1", "--format=%s"); !strings.Contains(subject, "refs/heads/x") {
		t.Errorf("subject = %q, want it to name refs/heads/x", subject)
	}
}

func TestMergeAcceptsRemoteBranchesAndTags(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/origin/side", "side")
	gittest.Run(t, repo.Dir, "tag", "-a", "-m", "v1", "v1", "side")
	mainTip := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	for _, tc := range []struct {
		ref  core.RefName
		want string
	}{
		{"refs/remotes/origin/side", "Merge remote-tracking branch 'origin/side'"},
		{"refs/tags/v1", "Merge tag 'v1'"},
	} {
		gittest.Run(t, repo.Dir, "reset", "--hard", string(mainTip))
		if _, err := c.Merge(t.Context(), wt, tc.ref, false); err != nil {
			t.Fatalf("Merge(%s): %v", tc.ref, err)
		}
		if subject := gittest.Out(t, repo.Dir, "log", "-1", "--format=%s"); !strings.HasPrefix(subject, tc.want) {
			t.Errorf("Merge(%s) subject = %q, want prefix %q", tc.ref, subject, tc.want)
		}
	}
}

func TestMergeOfAContainedBranchIsUpToDate(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "base.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "branch", "side")
	gittest.Commit(t, repo.Dir, "main.txt", "main\n", "main commit")
	head := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != core.IntegrateUpToDate {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateUpToDate)
	}
	if after := gittest.HeadSHA(t, repo.Dir); after != head {
		t.Errorf("HEAD moved to %q, want %q", after, head)
	}
}

func TestMergeBlockedByLocalChangesIsDirtyWorktree(t *testing.T) {
	t.Parallel()
	for _, diverged := range []bool{false, true} {
		repo := gittest.New(t)
		gittest.Commit(t, repo.Dir, "base.txt", "base\n", "base")
		gittest.Run(t, repo.Dir, "checkout", "-b", "side")
		gittest.Commit(t, repo.Dir, "base.txt", "side\n", "side commit")
		gittest.Run(t, repo.Dir, "checkout", "master")
		if diverged {
			gittest.Commit(t, repo.Dir, "main.txt", "main\n", "main commit")
		}
		writeFile(t, repo.Dir, "base.txt", "local\n")

		c, wt := openWorktree(t, repo.Dir)
		_, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
		ce := coretest.AsError(t, err, core.DirtyWorktree)
		if ce.Key != "mergeBlockedByChanges" {
			t.Errorf("diverged=%v: Key = %q, want mergeBlockedByChanges", diverged, ce.Key)
		}
		if got := readFile(t, repo.Dir, "base.txt"); got != "local\n" {
			t.Errorf("diverged=%v: base.txt = %q, want the local change", diverged, got)
		}
		if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
			t.Errorf("diverged=%v: stash list = %q, want empty", diverged, list)
		}
	}
}

func TestMergeStopsAsConflictedOrStopped(t *testing.T) {
	t.Parallel()
	repo := divergedConflictRepo(t)
	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Merge(t.Context(), wt, "refs/heads/side", false)
	coretest.AssertCode(t, err, core.Conflicted)
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Error("MERGE_HEAD missing, want the merge left for the conflict flow")
	}

	repo = divergedFixture(t)
	writeHook(t, repo.Dir, "pre-merge-commit", "exit 1")
	c, wt = openWorktree(t, repo.Dir)
	_, err = c.Merge(t.Context(), wt, "refs/heads/side", false)
	coretest.AssertCode(t, err, core.Stopped)
}

func TestRebaseFlattensMergesAndMovesNoOtherBranch(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"apply", "merge"} {
		t.Run(backend, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			shas := gittest.History(t, repo.Dir,
				gittest.Step{Message: "base", Files: map[string]string{"base.txt": "base\n"}},
				gittest.Step{Branch: "other", From: "master", Message: "other commit", Files: map[string]string{"other.txt": "other\n"}},
				gittest.Step{Branch: "feat", From: "master", Message: "feat 1", Files: map[string]string{"f1.txt": "1\n"}},
				gittest.Step{Branch: "feat", Message: "feat 2", Files: map[string]string{"f2.txt": "2\n"}},
				gittest.Step{Branch: "feat", Merge: "other", Message: "merge other", Files: map[string]string{"other.txt": "other\n"}},
				gittest.Step{Message: "main commit", Files: map[string]string{"main.txt": "main\n"}},
			)
			midTip := shas[2]
			gittest.Run(t, repo.Dir, "branch", "mid", string(midTip))
			gittest.Run(t, repo.Dir, "checkout", "feat")
			gittest.Run(t, repo.Dir, "config", "rebase.rebaseMerges", "true")
			gittest.Run(t, repo.Dir, "config", "rebase.updateRefs", "true")
			gittest.Run(t, repo.Dir, "config", "rebase.backend", backend)

			c, wt := openWorktree(t, repo.Dir)
			got, err := c.Rebase(t.Context(), wt, "refs/heads/master", false)
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != core.IntegrateRebased {
				t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateRebased)
			}
			gittest.Run(t, repo.Dir, "merge-base", "--is-ancestor", "master", "HEAD")
			if merges := gittest.Out(t, repo.Dir, "rev-list", "--merges", "master..HEAD"); merges != "" {
				t.Errorf("merge commits after the rebase = %q, want none", merges)
			}
			if mid := gittest.Out(t, repo.Dir, "rev-parse", "mid"); core.SHA(mid) != midTip {
				t.Errorf("mid = %q, want unchanged %q", mid, midTip)
			}
		})
	}
}

func TestRebaseOntoAnAncestorIsUpToDate(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "base.txt", "base\n", "base")
	gittest.Run(t, repo.Dir, "checkout", "-b", "feat")
	gittest.Commit(t, repo.Dir, "f1.txt", "1\n", "feat 1")
	head := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Rebase(t.Context(), wt, "refs/heads/master", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != core.IntegrateUpToDate {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateUpToDate)
	}
	if after := gittest.HeadSHA(t, repo.Dir); after != head {
		t.Errorf("HEAD moved to %q, want %q", after, head)
	}
}

func TestRebaseBlockedByLocalChangesIsDirtyWorktree(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "side")
	gittest.Run(t, repo.Dir, "config", "rebase.autoStash", "true")
	writeFile(t, repo.Dir, "side.txt", "local\n")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Rebase(t.Context(), wt, "refs/heads/master", false)
	if ce := coretest.AsError(t, err, core.DirtyWorktree); ce.Key != "rebaseBlockedByChanges" {
		t.Errorf("Key = %q, want rebaseBlockedByChanges", ce.Key)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
}

func TestRebaseStopsOnConflictsLeavingTheRebase(t *testing.T) {
	t.Parallel()
	repo := divergedConflictRepo(t)
	gittest.Run(t, repo.Dir, "checkout", "side")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Rebase(t.Context(), wt, "refs/heads/master", false)
	coretest.AssertCode(t, err, core.Conflicted)
	if !fileExists(filepath.Join(repo.Dir, ".git", "rebase-merge")) {
		t.Error("rebase-merge missing, want the rebase left for the conflict flow")
	}
}

const tenNumberLines = "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"

// distantEditsFixture is a repo on master where side changes the first line
// of base.txt, master moves on in main.txt, and the worktree has an unstaged
// change to the last line of base.txt: it blocks a merge or rebase of side
// but merges cleanly with side's change.
func distantEditsFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := diverge(t, tenNumberLines, edit{"base.txt", strings.Replace(tenNumberLines, "1\n", "one\n", 1)}, edit{"main.txt", "main\n"})
	writeFile(t, repo.Dir, "base.txt", strings.Replace(tenNumberLines, "10\n", "ten\n", 1))
	return repo
}

const mergedWithLocalChange = "one\n2\n3\n4\n5\n6\n7\n8\n9\nten\n"

func TestMergeWithStashRestoresTheLocalChange(t *testing.T) {
	t.Parallel()
	repo := distantEditsFixture(t)

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Merge(t.Context(), wt, "refs/heads/side", true)
	if err != nil {
		t.Fatal(err)
	}

	if got.Outcome != core.IntegrateMerged {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateMerged)
	}
	if content := readFile(t, repo.Dir, "base.txt"); content != mergedWithLocalChange {
		t.Errorf("base.txt = %q, want the merge and the local change", content)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
}

func TestMergeWithStashKeepsTheEntryOnConflicts(t *testing.T) {
	t.Parallel()
	repo := divergedConflictRepo(t)
	gittest.Commit(t, repo.Dir, "other.txt", "original\n", "add other.txt")
	writeFile(t, repo.Dir, "other.txt", "local\n")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Merge(t.Context(), wt, "refs/heads/side", true)

	coretest.AssertCode(t, err, core.Conflicted)
	list := gittest.Out(t, repo.Dir, "stash", "list")
	if !strings.HasSuffix(list, ": Gärtli: local changes before merge") || strings.Contains(list, "\n") {
		t.Errorf("stash list = %q, want one entry with subject %q", list, "Gärtli: local changes before merge")
	}
	summary, err := c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if want := gittest.Out(t, repo.Dir, "rev-parse", "refs/stash"); summary.WaitingStash != core.SHA(want) {
		t.Errorf("Summary.WaitingStash = %q, want %q", summary.WaitingStash, want)
	}
}

func TestWaitingStashFindsAnEntryWithTheOlderPullSubject(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "changed\n")
	sha := stashPush(t, repo.Dir, "Gärtli: local changes before pull")

	c, wt := openWorktree(t, repo.Dir)
	summary, err := c.Summary(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	if summary.WaitingStash != core.SHA(sha) {
		t.Errorf("Summary.WaitingStash = %q, want %q", summary.WaitingStash, sha)
	}
}

func TestRebaseWithStashRestoresTheLocalChange(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "side")
	writeFile(t, repo.Dir, "side.txt", "local\n")

	c, wt := openWorktree(t, repo.Dir)
	got, err := c.Rebase(t.Context(), wt, "refs/heads/master", true)
	if err != nil {
		t.Fatal(err)
	}

	if got.Outcome != core.IntegrateRebased {
		t.Errorf("Outcome = %q, want %q", got.Outcome, core.IntegrateRebased)
	}
	if content := readFile(t, repo.Dir, "side.txt"); content != "local\n" {
		t.Errorf("side.txt = %q, want the local change kept", content)
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
}

func TestRebaseWithStashKeepsTheEntryWhenAFailedRebaseLeftTheTreeChanged(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "side")
	writeFile(t, repo.Dir, "side.txt", "local\n")
	writeHook(t, repo.Dir, "pre-rebase", "echo hooked > side.txt\ngit add side.txt\nexit 1\n")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Rebase(t.Context(), wt, "refs/heads/master", true)

	if err == nil {
		t.Fatal("Rebase succeeded, want the hook to stop it")
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list == "" {
		t.Error("stash list is empty, want the entry kept")
	}
	if status := gittest.Out(t, repo.Dir, "status", "--porcelain", "--untracked-files=no"); strings.Contains(status, "UU") {
		t.Errorf("status = %q, want no conflicted file", status)
	}
}

func TestRebaseWithStashGivesTheEntryBackWhenTheFailedRebaseOnlyTouchedFiles(t *testing.T) {
	t.Parallel()
	repo := divergedFixture(t)
	gittest.Run(t, repo.Dir, "checkout", "side")
	writeFile(t, repo.Dir, "side.txt", "local\n")
	writeHook(t, repo.Dir, "pre-rebase", "touch -d 2001-01-01 side.txt\nexit 1\n")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Rebase(t.Context(), wt, "refs/heads/master", true)

	if err == nil {
		t.Fatal("Rebase succeeded, want the hook to stop it")
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
	if got := readFile(t, repo.Dir, "side.txt"); got != "local\n" {
		t.Errorf("side.txt = %q, want the local change back", got)
	}
}

func TestMergeWithStashGivesTheEntryBackWhenTheMergeNeverStarted(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f", "base\n", "base")
	gittest.Run(t, repo.Dir, "checkout", "-b", "side")
	gittest.Commit(t, repo.Dir, "u", "side\n", "side adds u")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "main.txt", "main\n", "main commit")
	writeFile(t, repo.Dir, "f", "local\n")
	writeFile(t, repo.Dir, "u", "untracked\n")

	c, wt := openWorktree(t, repo.Dir)
	_, err := c.Merge(t.Context(), wt, "refs/heads/side", true)

	if err == nil {
		t.Fatal("Merge succeeded, want the untracked file to stop it")
	}
	if list := gittest.Out(t, repo.Dir, "stash", "list"); list != "" {
		t.Errorf("stash list = %q, want empty", list)
	}
	if content := readFile(t, repo.Dir, "f"); content != "local\n" {
		t.Errorf("f = %q, want the local change back", content)
	}
}
