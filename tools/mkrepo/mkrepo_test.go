package mkrepo

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/git"
)

// smallConfig is a fast-to-build stand-in for Params, big enough to exercise
// every code path (branches, merges, tags, worktrees) at unit-test speed.
func smallConfig(seed int64) Config {
	return Config{Commits: 200, FeatureBranches: 4, TagCount: 3, MergeEvery: 50, Worktrees: 2, Seed: seed}
}

func TestBuildIsDeterministicGivenTheSameConfig(t *testing.T) {
	cfg := smallConfig(7)
	dirA := filepath.Join(t.TempDir(), "a")
	dirB := filepath.Join(t.TempDir(), "b")

	if err := Build(t.Context(), dirA, cfg); err != nil {
		t.Fatal(err)
	}
	if err := Build(t.Context(), dirB, cfg); err != nil {
		t.Fatal(err)
	}

	if headSHA(t, dirA) != headSHA(t, dirB) {
		t.Errorf("HEAD SHA differs between builds")
	}
	if got, want := refSHAs(t, dirA), refSHAs(t, dirB); got != want {
		t.Errorf("ref SHA sets differ:\n%s\nvs\n%s", got, want)
	}
	if commitCount(t, dirA) != commitCount(t, dirB) {
		t.Errorf("commit counts differ")
	}
}

func TestBuildProducesAWellFormedRepoMatchingConfig(t *testing.T) {
	cfg := smallConfig(11)
	dir := filepath.Join(t.TempDir(), "repo")
	if err := Build(t.Context(), dir, cfg); err != nil {
		t.Fatal(err)
	}

	if got := commitCount(t, dir); got != cfg.Commits {
		t.Errorf("rev-list --all --count = %d, want %d", got, cfg.Commits)
	}
	if got := len(refNames(t, dir, "refs/heads")); got != cfg.FeatureBranches+1 { // +1 for main
		t.Errorf("branches = %d, want %d feature branches + main", got, cfg.FeatureBranches)
	}
	if got := len(refNames(t, dir, "refs/tags")); got != cfg.TagCount {
		t.Errorf("tags = %d, want %d", got, cfg.TagCount)
	}
	if got := len(worktrees(t, dir)); got != cfg.Worktrees+1 { // +1 for the primary worktree
		t.Errorf("worktrees = %d, want %d extra + the primary one", got, cfg.Worktrees)
	}

	run(t, dir, "fsck")
}

func TestBuildIsACacheHitWhenTheConfigIsUnchanged(t *testing.T) {
	cfg := smallConfig(3)
	dir := filepath.Join(t.TempDir(), "repo")
	if err := Build(t.Context(), dir, cfg); err != nil {
		t.Fatal(err)
	}

	calls := 0
	prev := runner
	runner.Log = func(e git.Entry) {
		if strings.HasPrefix(e.Command, "git fast-import") {
			calls++
		}
	}
	t.Cleanup(func() { runner = prev })

	if err := Build(t.Context(), dir, cfg); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("fast-import ran %d times on a cache hit, want 0", calls)
	}
}

func TestBuildRebuildsWhenTheConfigChanges(t *testing.T) {
	cfg := smallConfig(5)
	dir := filepath.Join(t.TempDir(), "repo")
	if err := Build(t.Context(), dir, cfg); err != nil {
		t.Fatal(err)
	}

	cfg.Commits = 210
	if err := Build(t.Context(), dir, cfg); err != nil {
		t.Fatal(err)
	}

	if got := commitCount(t, dir); got != 210 {
		t.Errorf("rev-list --all --count = %d, want 210 after rebuild", got)
	}
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(runOut(t, dir, "rev-parse", "HEAD"))
}

// refSHAs lists every ref with its object, sorted by refname.
func refSHAs(t *testing.T, dir string) string {
	t.Helper()
	return runOut(t, dir, "for-each-ref", "--format=%(refname) %(objectname)")
}

func commitCount(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(runOut(t, dir, "rev-list", "--all", "--count")))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func refNames(t *testing.T, dir, prefix string) []string {
	t.Helper()
	return nonEmptyLines(runOut(t, dir, "for-each-ref", "--format=%(refname)", prefix))
}

func worktrees(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	for _, l := range nonEmptyLines(runOut(t, dir, "worktree", "list", "--porcelain")) {
		if p, ok := strings.CutPrefix(l, "worktree "); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

func nonEmptyLines(s string) []string {
	var lines []string
	for l := range strings.SplitSeq(strings.TrimSpace(s), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func runOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	result, err := runner.Run(t.Context(), git.Call{Dir: dir, ReadOnly: true, Args: args})
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(result.Stdout)
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := runner.Run(t.Context(), git.Call{Dir: dir, Args: args}); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}
