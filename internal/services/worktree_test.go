package services

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_WT_01_ParseWorktreeRecordsParsesFieldsAndFlags(t *testing.T) {
	t.Parallel()
	fixture := "worktree /repo\x00HEAD 1111111111111111111111111111111111111111\x00branch refs/heads/main\x00\x00" +
		"worktree /repo-detached\x00HEAD 2222222222222222222222222222222222222222\x00detached\x00\x00" +
		"worktree /repo-locked\x00HEAD 3333333333333333333333333333333333333333\x00branch refs/heads/locked-branch\x00locked reason\x00\x00" +
		"worktree /repo-prunable\x00HEAD 4444444444444444444444444444444444444444\x00detached\x00prunable gitdir file points to non-existent location\x00\x00"

	got := parseWorktreeRecords([]byte(fixture))

	if len(got) != 4 {
		t.Fatalf("len(records) = %d, want 4", len(got))
	}
	for i, r := range got {
		if r.Main != (i == 0) {
			t.Errorf("records[%d].Main = %v, want only the first, the main worktree", i, r.Main)
		}
	}
	if got[0].Branch != "main" {
		t.Errorf("branch worktree = %+v, want Branch \"main\"", got[0])
	}
	if got[1].Branch != "" {
		t.Errorf("detached worktree = %+v, want Branch empty", got[1])
	}
	if !got[2].Locked {
		t.Errorf("locked worktree = %+v, want Locked true", got[2])
	}
	if !got[3].Missing {
		t.Errorf("prunable worktree = %+v, want Missing true", got[3])
	}
}

func TestFR_WT_01_ListWorktreesReportsEveryWorktreeShape(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "branch", "dirty")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linkedDir, "feature")
	detachedDir := filepath.Join(t.TempDir(), "detached")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", detachedDir, sha)
	dirtyDir := filepath.Join(t.TempDir(), "dirty")
	gittest.Run(t, repo.Dir, "worktree", "add", dirtyDir, "dirty")
	writeFile(t, dirtyDir, "seed.txt", "changed")
	missingDir := filepath.Join(t.TempDir(), "missing")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", missingDir, sha)
	paths := map[string]string{
		"main": gitPath(t, repo.Dir), "linked": gitPath(t, linkedDir), "detached": gitPath(t, detachedDir),
		"dirty": gitPath(t, dirtyDir), "missing": gitPath(t, missingDir),
	}
	if err := os.RemoveAll(missingDir); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("main first, linked ones not main", func(t *testing.T) {
		if len(got) != len(paths) {
			t.Fatalf("len(worktrees) = %d, want %d: %+v", len(got), len(paths), got)
		}
		if !got[0].Main || got[0].Path != paths["main"] {
			t.Errorf("worktrees[0] = %+v, want the main worktree at %s", got[0], paths["main"])
		}
		for i := 1; i < len(got); i++ {
			if got[i].Main {
				t.Errorf("worktrees[%d].Main = true, want false", i)
			}
		}
	})
	t.Run("branch or detached", func(t *testing.T) {
		if linked := findWorktree(t, got, paths["linked"]); linked.Branch != "feature" {
			t.Errorf("linked worktree = %+v, want Branch \"feature\"", linked)
		}
		if detached := findWorktree(t, got, paths["detached"]); detached.Branch != "" {
			t.Errorf("detached worktree = %+v, want Branch empty", detached)
		}
	})
	t.Run("dirty per worktree", func(t *testing.T) {
		if wt := findWorktree(t, got, paths["dirty"]); !wt.Dirty {
			t.Error("dirty worktree Dirty = false, want true")
		}
		if wt := findWorktree(t, got, paths["detached"]); wt.Dirty {
			t.Error("clean worktree Dirty = true, want false")
		}
	})
	t.Run("missing directory without error", func(t *testing.T) {
		wt := findWorktree(t, got, paths["missing"])
		if !wt.Missing {
			t.Error("Missing = false, want true")
		}
		if wt.Dirty {
			t.Error("Dirty = true, want false (a missing worktree is never status-checked)")
		}
	})
	t.Run("main worktree found from the common git dir", func(t *testing.T) {
		main, err := c.MainWorktree(t.Context(), core.RepoPath(filepath.Join(repo.Dir, ".git")))
		if err != nil {
			t.Fatal(err)
		}
		if !main.Main || main.Path != paths["main"] {
			t.Errorf("MainWorktree = %+v, want the main worktree at %s", main, paths["main"])
		}
	})
}

func TestFR_WT_13_MainWorktreeErrorsForABareRepoWhoseOnlyWorktreeIsGone(t *testing.T) {
	t.Parallel()
	bareDir, linkedDir, _ := bareRepoWithWorktree(t)
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

func TestFR_WT_01_ListWorktreesMarksABareRepoAndRunsNoStatusInIt(t *testing.T) {
	t.Parallel()
	bareDir, linkedDir, _ := bareRepoWithWorktree(t)
	c := New(func(core.Event) {})
	rec := &commandRecorder{}
	c.discover.Log = rec.record

	got, err := c.ListWorktrees(t.Context(), core.RepoPath(linkedDir))
	if err != nil {
		t.Fatal(err)
	}

	barePath := gitPath(t, bareDir)
	if len(got) != 2 || !got[0].Bare || got[0].Path != barePath || got[0].Dirty || got[1].Bare {
		t.Fatalf("worktrees = %+v, want the bare repo first, marked bare, then the linked worktree", got)
	}
	for _, e := range rec.snapshot() {
		if e.Dir == barePath && strings.Contains(e.Command, "status") {
			t.Errorf("ran %q in the bare repo, want no status there", e.Command)
		}
	}
}

func TestFR_UX_11_MainWorktreeNamesThePathInArgsWhenGitCannotListIt(t *testing.T) {
	t.Parallel()
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

func TestFR_WT_02_AddWorktreeChecksOutAnExistingBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "wt")

	c, _ := openWorktree(t, repo.Dir)
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if want := gitPath(t, target); got.Path != want || got.Branch != "feature" {
		t.Errorf("AddWorktree = %+v, want Path %s, Branch \"feature\"", got, want)
	}
	if branch := currentBranch(t, target); branch != "feature" {
		t.Errorf("checked-out branch in %s = %q, want feature", target, branch)
	}
}

func TestFR_WT_14_AddWorktreeFindsTheNewWorktreeGivenAnotherFormOfItsPath(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(anotherFormOfATempDir(t), "wt")

	c, _ := openWorktree(t, repo.Dir)
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if want := gitPath(t, target); got.Path != want {
		t.Errorf("AddWorktree Path = %q, want git's form %q", got.Path, want)
	}
}

// bareRepoWithWorktree clones a one-commit repo bare, its HEAD naming
// branch head, and adds a linked worktree on a new branch "other".
func bareRepoWithWorktree(t *testing.T) (bareDir, linkedDir, head string) {
	t.Helper()
	src := gittest.New(t)
	gittest.Commit(t, src.Dir, "seed.txt", "a", "first")
	bareDir = filepath.Join(t.TempDir(), "bare.git")
	gittest.Run(t, src.Dir, "clone", "-q", "--bare", src.Dir, bareDir)
	gittest.Run(t, bareDir, "branch", "other")
	linkedDir = filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, bareDir, "worktree", "add", "-q", linkedDir, "other")
	return bareDir, linkedDir, currentBranch(t, src.Dir)
}

// anotherFormOfATempDir returns a temp folder that git resolves to a
// different string than the one given to it: on Windows, t.TempDir()'s own
// short, backslash form; elsewhere, a folder reached through a symlink.
func anotherFormOfATempDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return t.TempDir()
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	return link
}

func TestFR_WT_02_AddWorktreeCreatesANewBranchFromAGivenStart(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	firstSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "b", "second")
	target := filepath.Join(t.TempDir(), "wt")

	c, _ := openWorktree(t, repo.Dir)
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

func TestFR_WT_02_AddWorktreeWithANewBranchFromARemoteBranchSetsNoUpstream(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	withOrigin(t, repo.Dir, "master")
	originTip := gittest.Out(t, repo.Dir, "rev-parse", "refs/remotes/origin/master")
	target := filepath.Join(t.TempDir(), "wt")

	c, _ := openWorktree(t, repo.Dir)
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "topic", "refs/remotes/origin/master", true)
	if err != nil {
		t.Fatal(err)
	}

	if got.Branch != "topic" || got.SHA != originTip {
		t.Errorf("AddWorktree = %+v, want Branch \"topic\" at %s", got, originTip)
	}
	if branch := currentBranch(t, target); branch != "topic" {
		t.Errorf("checked-out branch in %s = %q, want topic", target, branch)
	}
	if upstream := upstreamOf(t, repo.Dir, "topic"); upstream != "" {
		t.Errorf("upstream = %q, want none", upstream)
	}
}

func TestFR_WT_02_AddWorktreeStartsANewBranchAtTheFullRefNamedWhenATagAndABranchShareItsName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	branchSHA, tagSHA := branchAndTagNamedV1(t, repo.Dir)

	c, _ := openWorktree(t, repo.Dir)
	for _, tc := range []struct{ name, start, want string }{
		{"from-branch", "refs/heads/v1", branchSHA},
		{"from-tag", "refs/tags/v1", tagSHA},
	} {
		target := filepath.Join(t.TempDir(), tc.name)
		if _, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, tc.name, tc.start, true); err != nil {
			t.Fatalf("AddWorktree(%s, %s): %v", tc.name, tc.start, err)
		}
		if got := gittest.HeadSHA(t, target); got != tc.want {
			t.Errorf("%s's HEAD = %s, want %s's commit %s", tc.name, got, tc.start, tc.want)
		}
	}
}

func TestFR_WT_02_AddWorktreeWithANewBranchAndNoStartBranchesFromHEAD(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	head := gittest.HeadSHA(t, repo.Dir)
	target := filepath.Join(t.TempDir(), "wt")

	c, _ := openWorktree(t, repo.Dir)
	if _, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "fresh", "", true); err != nil {
		t.Fatal(err)
	}

	if got := gittest.HeadSHA(t, target); got != head {
		t.Errorf("new worktree HEAD = %s, want the repo's HEAD %s", got, head)
	}
}

func TestFR_WT_02_AddWorktreeRefusalsCreateNeitherBranchNorFolder(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "branch", "dup")
	sha := gittest.HeadSHA(t, repo.Dir)
	dupTip := branchTip(t, repo.Dir, "dup")
	branchesBefore := gittest.Out(t, repo.Dir, "branch", "--list")
	occupied := t.TempDir()
	writeFile(t, occupied, "existing.txt", "x")
	free := filepath.Join(t.TempDir(), "wt")

	c, _ := openWorktree(t, repo.Dir)
	add := func(target, branch, start string, create bool) error {
		_, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, branch, start, create)
		return err
	}

	t.Run("non-empty target with an existing branch", func(t *testing.T) {
		ce := coretest.AsError(t, add(occupied, "feature", "", false), core.GitFailed)
		if !strings.Contains(ce.GitStderr, "already exists") {
			t.Errorf("GitStderr = %q, want it to say %s already exists", ce.GitStderr, occupied)
		}
	})
	t.Run("non-empty target with a new branch", func(t *testing.T) {
		ce := coretest.AsError(t, add(occupied, "stray", "", true), core.GitFailed)
		if ce.Args["path"] != occupied || ce.Key != "worktreeExists" {
			t.Errorf("Key, Args = %q, %v; want worktreeExists, path=%s", ce.Key, ce.Args, occupied)
		}
	})
	t.Run("invalid new branch name", func(t *testing.T) {
		coretest.AssertCode(t, add(free, "-weird", "", true), core.InvalidName)
	})
	t.Run("duplicate new branch name", func(t *testing.T) {
		coretest.AssertCode(t, add(free, "dup", sha, true), core.GitFailed)
	})

	entries, err := os.ReadDir(occupied)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "existing.txt" {
		t.Errorf("occupied dir = %v, want unchanged (only existing.txt)", entries)
	}
	if _, statErr := os.Stat(free); !os.IsNotExist(statErr) {
		t.Errorf("target %s exists, want none created", free)
	}
	if got := gittest.Out(t, repo.Dir, "branch", "--list"); got != branchesBefore {
		t.Errorf("branches = %q, want unchanged %q", got, branchesBefore)
	}
	if got := branchTip(t, repo.Dir, "dup"); got != dupTip {
		t.Errorf("dup = %s, want unchanged %s", got, dupTip)
	}
}

func TestFR_WT_03_AddWorktreeCreatesTheTargetsMissingParentDirectory(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	target := filepath.Join(t.TempDir(), "missing-parent", "wt") // "missing-parent" does not exist yet

	c, _ := openWorktree(t, repo.Dir)
	got, err := c.AddWorktree(t.Context(), core.RepoPath(repo.Dir), target, "feature", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if want := gitPath(t, target); got.Path != want {
		t.Errorf("AddWorktree Path = %s, want %s", got.Path, want)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("target %s missing: %v (git should have created its own parent directory)", target, statErr)
	}
}

func TestFR_WT_07_RemoveWorktreeRefusesLockedAndDirtyThenForceRemovesDirty(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "locked")
	gittest.Run(t, repo.Dir, "branch", "dirty")
	locked := filepath.Join(t.TempDir(), "locked")
	gittest.Run(t, repo.Dir, "worktree", "add", locked, "locked")
	gittest.Run(t, repo.Dir, "worktree", "lock", locked)
	dirty := filepath.Join(t.TempDir(), "dirty")
	gittest.Run(t, repo.Dir, "worktree", "add", dirty, "dirty")
	writeFile(t, dirty, "seed.txt", "changed")

	c, _ := openWorktree(t, repo.Dir)
	remove := func(target string, force bool) error {
		return c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), target, force)
	}

	ce := coretest.AsError(t, remove(locked, true), core.GitFailed)
	if !strings.Contains(ce.GitStderr, "cannot remove a locked working tree") {
		t.Errorf("GitStderr = %q, want git's own locked-worktree message", ce.GitStderr)
	}
	if _, statErr := os.Stat(locked); statErr != nil {
		t.Errorf("worktree %s missing: %v, want still present", locked, statErr)
	}

	ce = coretest.AsError(t, remove(dirty, false), core.DirtyWorktree)
	if ce.Key != "worktreeDirty" || ce.Args["path"] != dirty {
		t.Errorf("key, args = %q, %v; want worktreeDirty, path=%s", ce.Key, ce.Args, dirty)
	}
	if !strings.Contains(ce.GitStderr, "contains modified or untracked files") {
		t.Errorf("GitStderr = %q, want git's own dirty-worktree message", ce.GitStderr)
	}
	if got := readFile(t, dirty, "seed.txt"); got != "changed" {
		t.Errorf("seed.txt = %q, want unchanged %q", got, "changed")
	}

	if err := remove(dirty, true); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(dirty); !os.IsNotExist(statErr) {
		t.Errorf("worktree %s = %v, want removed", dirty, statErr)
	}
}

func TestFR_WT_07_RemoveWorktreeRemovesCleanAndMissingOnesAndPruneKeepsTheRest(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	sha := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	clean := filepath.Join(t.TempDir(), "clean")
	gittest.Run(t, repo.Dir, "worktree", "add", clean, "feature")
	keep := filepath.Join(t.TempDir(), "keep")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", keep, sha)
	missingRemoved := filepath.Join(t.TempDir(), "missing-removed")
	missingPruned := filepath.Join(t.TempDir(), "missing-pruned")
	for _, dir := range []string{missingRemoved, missingPruned} {
		gittest.Run(t, repo.Dir, "worktree", "add", "--detach", dir, sha)
	}
	cleanPath, keepPath := gitPath(t, clean), gitPath(t, keep)
	removedPath, prunedPath := gitPath(t, missingRemoved), gitPath(t, missingPruned)
	for _, dir := range []string{missingRemoved, missingPruned} {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}

	c, _ := openWorktree(t, repo.Dir)
	listed := func() []core.Worktree {
		got, err := c.ListWorktrees(t.Context(), core.RepoPath(repo.Dir))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	has := func(path string) bool {
		return slices.ContainsFunc(listed(), func(wt core.Worktree) bool { return wt.Path == path })
	}

	if err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), clean, false); err != nil {
		t.Fatal(err)
	}
	if has(cleanPath) {
		t.Errorf("worktrees = %+v, want the clean one gone", listed())
	}

	if err := c.RemoveWorktree(t.Context(), core.RepoPath(repo.Dir), missingRemoved, false); err != nil {
		t.Fatal(err)
	}
	if has(removedPath) {
		t.Errorf("worktrees = %+v, want the missing one removed without force", listed())
	}

	if err := c.PruneWorktrees(t.Context(), core.RepoPath(repo.Dir)); err != nil {
		t.Fatal(err)
	}
	if has(prunedPath) {
		t.Errorf("worktrees = %+v, want the missing one pruned away", listed())
	}
	if !has(keepPath) {
		t.Errorf("worktrees = %+v, want the present one kept by prune", listed())
	}
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

// gitPath returns dir's path the way git reports it: a worktree's toplevel,
// or a bare repo's own directory. Called before dir is removed.
func gitPath(t *testing.T, dir string) string {
	t.Helper()
	if gittest.Out(t, dir, "rev-parse", "--is-bare-repository") == "true" {
		return gittest.Out(t, dir, "rev-parse", "--absolute-git-dir")
	}
	return gittest.Out(t, dir, "rev-parse", "--show-toplevel")
}
