package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_WT_01_ParseWorktreeRecordsParsesFieldsAndFlags(t *testing.T) {
	fixture := "worktree /repo\x00HEAD 1111111111111111111111111111111111111111\x00branch refs/heads/main\x00\x00" +
		"worktree /repo-detached\x00HEAD 2222222222222222222222222222222222222222\x00detached\x00\x00" +
		"worktree /repo-locked\x00HEAD 3333333333333333333333333333333333333333\x00branch refs/heads/locked-branch\x00locked reason\x00\x00" +
		"worktree /repo-prunable\x00HEAD 4444444444444444444444444444444444444444\x00detached\x00prunable gitdir file points to non-existent location\x00\x00"

	got := parseWorktreeRecords([]byte(fixture))

	if len(got) != 4 {
		t.Fatalf("len(records) = %d, want 4", len(got))
	}
	if !got[0].Main {
		t.Error("records[0].Main = false, want true (the main worktree, listed first)")
	}
	for i := 1; i < len(got); i++ {
		if got[i].Main {
			t.Errorf("records[%d].Main = true, want false", i)
		}
	}

	branch := got[0]
	if branch.Branch != "main" || branch.Detached {
		t.Errorf("branch worktree = %+v, want Branch \"main\", not detached", branch)
	}

	detached := got[1]
	if !detached.Detached || detached.Branch != "" {
		t.Errorf("detached worktree = %+v, want Detached true, Branch empty", detached)
	}

	locked := got[2]
	if !locked.Locked {
		t.Errorf("locked worktree = %+v, want Locked true", locked)
	}

	prunable := got[3]
	if !prunable.Missing {
		t.Errorf("prunable worktree = %+v, want Missing true", prunable)
	}
}

func TestFR_WT_13_ParseBranchSummariesParsesCheckedOutIn(t *testing.T) {
	fixture := "main\x1f/repo\x00" + "\nfeature\x1f\x00"

	got := parseBranchSummaries([]byte(fixture))

	want := []core.BranchSummary{
		{Name: "main", CheckedOutIn: "/repo"},
		{Name: "feature", CheckedOutIn: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("summaries = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("summaries[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestFR_WT_01_ListWorktreesListsMainAndLinkedWorktrees(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	sha := gittest.HeadSHA(t, repo.Dir)
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linkedDir, "feature")
	detachedDir := filepath.Join(t.TempDir(), "detached")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", detachedDir, sha)

	c := New(func(core.Event) {})
	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("len(worktrees) = %d, want 3: %+v", len(got), got)
	}
	if !got[0].Main || got[0].Path != repo.Dir {
		t.Errorf("worktrees[0] = %+v, want the main worktree at %s", got[0], repo.Dir)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Main {
			t.Errorf("worktrees[%d].Main = true, want false", i)
		}
	}

	linked := findWorktree(t, got, linkedDir)
	if linked.Branch != "feature" || linked.Detached {
		t.Errorf("linked worktree = %+v, want Branch \"feature\", not detached", linked)
	}
	detached := findWorktree(t, got, detachedDir)
	if !detached.Detached || detached.Branch != "" {
		t.Errorf("detached worktree = %+v, want Detached true, Branch empty", detached)
	}
}

func TestFR_WT_01_ListWorktreesReportsDirtyPerWorktree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	dirtyDir := filepath.Join(t.TempDir(), "dirty")
	gittest.Run(t, repo.Dir, "worktree", "add", dirtyDir, "feature")
	writeFile(t, dirtyDir, "seed.txt", "changed")
	cleanDir := filepath.Join(t.TempDir(), "clean")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", cleanDir, sha)

	c := New(func(core.Event) {})
	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if wt := findWorktree(t, got, dirtyDir); !wt.Dirty {
		t.Errorf("dirty worktree Dirty = false, want true")
	}
	if wt := findWorktree(t, got, cleanDir); wt.Dirty {
		t.Errorf("clean worktree Dirty = true, want false")
	}
}

func TestFR_WT_01_ListWorktreesReportsAMissingDirectoryWithoutError(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linkedDir, gittest.HeadSHA(t, repo.Dir))
	if err := os.RemoveAll(linkedDir); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	wt := findWorktree(t, got, linkedDir)
	if !wt.Missing {
		t.Errorf("Missing = false, want true")
	}
	if wt.Dirty {
		t.Errorf("Dirty = true, want false (a missing worktree is never status-checked)")
	}
}

func TestFR_WT_13_MainWorktreeFindsTheMainWorktreeFromTheCommonGitDir(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linkedDir, gittest.HeadSHA(t, repo.Dir))
	if err := os.RemoveAll(linkedDir); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(repo.Dir, ".git")

	c := New(func(core.Event) {})
	got, err := c.MainWorktree(t.Context(), core.RepoPath(gitDir))
	if err != nil {
		t.Fatal(err)
	}

	if !got.Main || got.Path != repo.Dir {
		t.Errorf("MainWorktree = %+v, want the main worktree at %s", got, repo.Dir)
	}
}

func TestFR_WT_13_MainWorktreeErrorsForABareRepoWhoseOnlyWorktreeIsGone(t *testing.T) {
	src := gittest.New(t)
	gittest.Commit(t, src.Dir, "seed.txt", "a", "first")
	branch := gittest.Out(t, src.Dir, "branch", "--show-current")
	bareDir := filepath.Join(t.TempDir(), "bare.git")
	gittest.Run(t, src.Dir, "clone", "-q", "--bare", src.Dir, bareDir)
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, bareDir, "worktree", "add", "-q", linkedDir, branch)
	if err := os.RemoveAll(linkedDir); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	_, err := c.MainWorktree(t.Context(), core.RepoPath(bareDir))

	ce := coretest.AsError(t, err, core.NotARepo)
	if ce.Args["path"] != bareDir {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], bareDir)
	}
	if ce.Key != "noWorktreeLeft" {
		t.Errorf("Key = %q, want noWorktreeLeft", ce.Key)
	}
}

func TestFR_UX_11_MainWorktreeNamesThePathInArgsWhenGitCannotListIt(t *testing.T) {
	c := New(func(core.Event) {})
	bogus := core.RepoPath(t.TempDir())

	_, err := c.MainWorktree(t.Context(), bogus)

	ce := coretest.AsError(t, err, core.NotARepo)
	if ce.Args["path"] != string(bogus) {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], bogus)
	}
	if ce.Key != "notARepo" {
		t.Errorf("Key = %q, want notARepo", ce.Key)
	}
}

func TestFR_WT_13_ListBranchesReportsWhichWorktreeHasEachCheckedOut(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	mainBranch := currentBranch(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linkedDir, "feature")

	c := New(func(core.Event) {})
	got, err := c.ListBranches(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if feature := findBranchSummary(t, got, "feature"); feature.CheckedOutIn != linkedDir {
		t.Errorf("feature.CheckedOutIn = %q, want %q", feature.CheckedOutIn, linkedDir)
	}
	if main := findBranchSummary(t, got, mainBranch); main.CheckedOutIn != repo.Dir {
		t.Errorf("%s.CheckedOutIn = %q, want %q", mainBranch, main.CheckedOutIn, repo.Dir)
	}
}

func TestFR_WT_02_AddWorktreeChecksOutAnExistingBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "wt")

	c := New(func(core.Event) {})
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if got.Path != target || got.Branch != "feature" {
		t.Errorf("AddWorktree = %+v, want Path %s, Branch \"feature\"", got, target)
	}
	if branch := currentBranch(t, target); branch != "feature" {
		t.Errorf("checked-out branch in %s = %q, want feature", target, branch)
	}
}

func TestFR_WT_02_AddWorktreeCreatesANewBranchFromAGivenStart(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	target := filepath.Join(t.TempDir(), "wt")

	c := New(func(core.Event) {})
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "newb", firstSHA, true)
	if err != nil {
		t.Fatal(err)
	}

	if branchTip(t, repo.Dir, "newb") != firstSHA {
		t.Errorf("newb = %s, want %s", branchTip(t, repo.Dir, "newb"), firstSHA)
	}
	if got.Branch != "newb" || got.SHA != firstSHA {
		t.Errorf("AddWorktree = %+v, want Branch \"newb\" at %s", got, firstSHA)
	}
	if branch := currentBranch(t, target); branch != "newb" {
		t.Errorf("checked-out branch in %s = %q, want newb", target, branch)
	}
}

func TestFR_WT_02_AddWorktreeWithANewBranchAndNoStartBranchesFromHEAD(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	head := gittest.HeadSHA(t, repo.Dir)
	target := filepath.Join(t.TempDir(), "wt")

	c := New(func(core.Event) {})
	if _, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "fresh", "", true); err != nil {
		t.Fatal(err)
	}

	if got := gittest.HeadSHA(t, target); got != head {
		t.Errorf("new worktree HEAD = %s, want the repo's HEAD %s", got, head)
	}
}

func TestFR_WT_02_AddWorktreeIntoAFolderWithFilesCreatesNoBranch(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	target := t.TempDir()
	writeFile(t, target, "existing.txt", "x")

	c := New(func(core.Event) {})
	_, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "stray", "", true)

	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("AddWorktree err = %v, want an \"already exists\" refusal", err)
	}
	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.Args["path"] != target {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], target)
	}
	if ce.Key != "worktreeExists" {
		t.Errorf("Key = %q, want worktreeExists", ce.Key)
	}
	if out := gittest.Out(t, repo.Dir, "branch", "--list", "stray"); out != "" {
		t.Errorf("branch list = %q, want no stray branch", out)
	}
}

func TestFR_WT_02_AddWorktreeRejectsAnInvalidNewBranchNameBeforeTouchingGit(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	target := filepath.Join(t.TempDir(), "wt")

	c := New(func(core.Event) {})
	_, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "-weird", "", true)

	coretest.AssertCode(t, err, core.InvalidName)
	if branchExists(t, repo.Dir, "-weird") {
		t.Error("branch -weird was created, want none")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("target %s exists, want none created", target)
	}
}

func TestFR_WT_02_AddWorktreeRejectsADuplicateBranchNameAsGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "dup")
	originalTip := branchTip(t, repo.Dir, "dup")
	sha := gittest.HeadSHA(t, repo.Dir)
	target := filepath.Join(t.TempDir(), "wt")

	c := New(func(core.Event) {})
	_, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "dup", sha, true)

	coretest.AssertCode(t, err, core.GitFailed)
	if got := branchTip(t, repo.Dir, "dup"); got != originalTip {
		t.Errorf("dup = %s, want unchanged %s", got, originalTip)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("target %s exists, want none created", target)
	}
}

func TestFR_WT_03_AddWorktreeRefusesAnExistingNonEmptyTargetAsGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := t.TempDir()
	writeFile(t, target, "existing.txt", "x")

	c := New(func(core.Event) {})
	_, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "already exists") {
		t.Errorf("GitStderr = %q, want it to say %s already exists", ce.GitStderr, target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "existing.txt" {
		t.Errorf("target dir = %v, want unchanged (only existing.txt)", entries)
	}
}

func TestFR_WT_03_AddWorktreeCreatesTheTargetsMissingParentDirectory(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "missing-parent", "wt") // "missing-parent" does not exist yet

	c := New(func(core.Event) {})
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if got.Path != target {
		t.Errorf("AddWorktree Path = %s, want %s", got.Path, target)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("target %s missing: %v (git should have created its own parent directory)", target, statErr)
	}
}

func TestFR_WT_07_RemoveWorktreeRemovesACleanLinkedWorktree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", target, "feature")

	c := New(func(core.Event) {})
	if err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, false); err != nil {
		t.Fatal(err)
	}

	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range got {
		if wt.Path == target {
			t.Errorf("worktrees = %+v, want %s gone", got, target)
		}
	}
}

func TestFR_WT_07_RemoveWorktreeRefusesADirtyWorktreeAsCodeDirtyWorktree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", target, "feature")
	writeFile(t, target, "seed.txt", "changed")

	c := New(func(core.Event) {})
	err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, false)

	ce := coretest.AsError(t, err, core.DirtyWorktree)
	if !strings.Contains(ce.GitStderr, "contains modified or untracked files") {
		t.Errorf("GitStderr = %q, want git's own dirty-worktree message", ce.GitStderr)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("worktree %s missing: %v, want still present", target, statErr)
	}
	if got := readFile(t, target, "seed.txt"); got != "changed" {
		t.Errorf("seed.txt = %q, want unchanged %q", got, "changed")
	}
}

func TestFR_WT_07_RemoveWorktreeForcedRemovesADirtyWorktree(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", target, "feature")
	writeFile(t, target, "seed.txt", "changed")

	c := New(func(core.Event) {})
	if err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, true); err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Errorf("worktree %s = %v, want removed", target, statErr)
	}
}

func TestFR_WT_07_RemoveWorktreeRefusesALockedWorktreeEvenForced(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", target, "feature")
	gittest.Run(t, repo.Dir, "worktree", "lock", target)

	c := New(func(core.Event) {})
	err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, true)

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "cannot remove a locked working tree") {
		t.Errorf("GitStderr = %q, want git's own locked-worktree message", ce.GitStderr)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("worktree %s missing: %v, want still present", target, statErr)
	}
}

func TestFR_WT_07_RemoveWorktreeRemovesAMissingWorktreeWithNoForce(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	target := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", target, gittest.HeadSHA(t, repo.Dir))
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	if err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, false); err != nil {
		t.Fatal(err)
	}

	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range got {
		if wt.Path == target {
			t.Errorf("worktrees = %+v, want %s gone", got, target)
		}
	}
}

func TestFR_WT_07_PruneWorktreesRemovesOnlyMissingEntries(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	missing := filepath.Join(t.TempDir(), "missing")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", missing, sha)
	if err := os.RemoveAll(missing); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(t.TempDir(), "clean")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", clean, sha)

	c := New(func(core.Event) {})
	if err := c.PruneWorktrees(t.Context(), core.RepoPath(repo.Dir)); err != nil {
		t.Fatal(err)
	}

	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range got {
		if wt.Path == missing {
			t.Errorf("worktrees = %+v, want %s pruned away", got, missing)
		}
	}
	findWorktree(t, got, clean) // still present, fails the test if not
}

// findBranchSummary returns the entry of branches named name, failing t if
// none matches.
func findBranchSummary(t *testing.T, branches []core.BranchSummary, name string) core.BranchSummary {
	t.Helper()
	for _, b := range branches {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("no branch named %s in %+v", name, branches)
	return core.BranchSummary{}
}

// findWorktree returns the row of worktrees whose Path is path, failing t
// if none matches.
func findWorktree(t *testing.T, worktrees []core.Worktree, path string) core.Worktree {
	t.Helper()
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt
		}
	}
	t.Fatalf("no worktree with path %s in %+v", path, worktrees)
	return core.Worktree{}
}
