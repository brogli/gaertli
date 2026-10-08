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

// tryGit returns git's stdout for args in dir, untrimmed; nil if git fails, as a merge that stops
// on a conflict does.
func tryGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	result, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: dir, Args: args})
	if err != nil {
		return nil
	}
	return result.Stdout
}

// gitStops runs git with args in dir and fails the test unless git fails,
// as a merge, rebase or pick that stops on a conflict does.
func gitStops(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: dir, Args: args}); err == nil {
		t.Fatalf("git %s did not stop on a conflict", strings.Join(args, " "))
	}
}

// version is one side's file in stoppedOn; nil deletes the file.
type version []byte

// commitVersion commits v as path, or removes path for a nil v.
func commitVersion(t *testing.T, dir, path string, v version, msg string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if v == nil {
		gittest.Run(t, dir, "rm", "-q", "--", path)
	} else {
		if err := os.WriteFile(full, v, 0o644); err != nil {
			t.Fatal(err)
		}
		gittest.Run(t, dir, "add", "--", path)
	}
	gittest.CommitAll(t, dir, msg)
}

// divergedRepo makes a repo where master and branch side both changed path
// from base, with master checked out.
func divergedRepo(t *testing.T, path string, base, current, incoming version) string {
	t.Helper()
	dir := gittest.New(t).Dir
	step := func(branch, from, msg string, v version) gittest.Step {
		s := gittest.Step{Branch: branch, From: from, Message: msg}
		if v == nil {
			s.Delete = []string{path}
		} else {
			s.Files = map[string]string{path: string(v)}
		}
		return s
	}
	gittest.History(t, dir,
		step("master", "", "base", base),
		step("side", "master", "incoming change", incoming),
		step("master", "", "current change", current),
	)
	return dir
}

func readConflict(t *testing.T, c *Core, wt core.WorktreeID, path string) core.ConflictFile {
	t.Helper()
	got, err := c.Conflict(t.Context(), wt, path)
	if err != nil {
		t.Fatalf("Conflict(%s) = %v", path, err)
	}
	return got
}

func TestFR_CONF_05_ConflictTextReadsTheThreeSidesAndTheWorkingFile(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("one\nbase\n"), version("one\ncurrent\n"), version("one\nincoming\n"))

	got := readConflict(t, c, wt, "a.txt")

	if got.Kind != core.ConflictText || got.Path != "a.txt" {
		t.Fatalf("Kind, Path = %q, %q", got.Kind, got.Path)
	}
	if got.Base.Content != "one\nbase\n" || got.Current.Content != "one\ncurrent\n" || got.Incoming.Content != "one\nincoming\n" {
		t.Errorf("sides = %q, %q, %q", got.Base.Content, got.Current.Content, got.Incoming.Content)
	}
	if !got.Base.Present || !got.Current.Present || !got.Incoming.Present {
		t.Errorf("Present = %v %v %v, want all true", got.Base.Present, got.Current.Present, got.Incoming.Present)
	}
	working, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != string(working) || !strings.Contains(got.Result, "<<<<<<<") {
		t.Errorf("Result = %q, want the working file %q", got.Result, working)
	}
	if got.Fingerprint == "" {
		t.Error("Fingerprint is empty")
	}
	if got.MarkerSize != 7 || len(conflictRegionsOf(got)) != 1 {
		t.Fatalf("MarkerSize, regions = %d, %d", got.MarkerSize, len(conflictRegionsOf(got)))
	}
	assertRegion(t, got, 0, "base\n", "current\n", "incoming\n")
}

func TestFR_CONF_06_ConflictTextSendsLFForCRLF(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("one\r\nbase\r\n"), version("one\r\ncurrent\r\n"), version("one\r\nincoming\r\nlone\rcr\r\n"))

	got := readConflict(t, c, wt, "a.txt")

	if got.Base.Content != "one\nbase\n" || got.Current.Content != "one\ncurrent\n" || got.Incoming.Content != "one\nincoming\nlone\rcr\n" {
		t.Errorf("sides = %q, %q, %q", got.Base.Content, got.Current.Content, got.Incoming.Content)
	}
	if strings.Contains(got.Result, "\r\n") {
		t.Errorf("Result = %q, want LF only", got.Result)
	}
}

func TestFR_CONF_07_ConflictKindDeleted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		current, incoming version
		wantCurrent       bool
	}{
		{"incoming deleted", version("current\n"), nil, true},
		{"current deleted", nil, version("incoming\n"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, wt, _ := stoppedOn(t, "merge", "a.txt", version("base\n"), tc.current, tc.incoming)

			got := readConflict(t, c, wt, "a.txt")

			if got.Kind != core.ConflictDeleted {
				t.Fatalf("Kind = %q", got.Kind)
			}
			if !got.Base.Present || got.Current.Present != tc.wantCurrent || got.Incoming.Present == tc.wantCurrent {
				t.Errorf("Present base, current, incoming = %v %v %v", got.Base.Present, got.Current.Present, got.Incoming.Present)
			}
		})
	}
}

func TestFR_CONF_07_ConflictKindBinary(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.bin", version("base\x00"), version("cur\x00rent"), version("inc\x00oming"))

	got := readConflict(t, c, wt, "a.bin")

	if got.Kind != core.ConflictBinary {
		t.Errorf("Kind = %q", got.Kind)
	}
	if got.Current.Content != "" || got.Result != "" || !got.Current.Present || !got.Incoming.Present {
		t.Errorf("no content for a binary file, present sides: %+v", got)
	}
}

func TestFR_CONF_07_ConflictKindBinaryByAttribute(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, ".gitattributes", "*.txt merge=binary\n", "attributes")
	gittest.Commit(t, dir, "a.txt", "base\n", "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	gittest.Commit(t, dir, "a.txt", "incoming\n", "incoming")
	gittest.Run(t, dir, "checkout", "-q", "master")
	gittest.Commit(t, dir, "a.txt", "current\n", "current")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	if got.Kind != core.ConflictBinary {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestFR_CONF_07_ConflictKindSymlink(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	dir := repo.Dir
	link := func(target, msg string) {
		_ = os.Remove(filepath.Join(dir, "l"))
		if err := os.Symlink(target, filepath.Join(dir, "l")); err != nil {
			t.Fatal(err)
		}
		gittest.Run(t, dir, "add", "--", "l")
		gittest.CommitAll(t, dir, msg)
	}
	link("base", "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	link("incoming", "incoming")
	gittest.Run(t, dir, "checkout", "-q", "master")
	link("current", "current")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "l")

	if got.Kind != core.ConflictSymlink {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestFR_CONF_07_ConflictKindRenameOnEachPathOfARenameRename(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, "a.txt", "some\ncontent\nthat\nstays\nthe same\n", "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	gittest.Run(t, dir, "mv", "a.txt", "b.txt")
	gittest.CommitAll(t, dir, "rename to b")
	gittest.Run(t, dir, "checkout", "-q", "master")
	gittest.Run(t, dir, "mv", "a.txt", "c.txt")
	gittest.CommitAll(t, dir, "rename to c")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	for _, path := range []string{"a.txt", "b.txt", "c.txt"} {
		if got := readConflict(t, c, wt, path); got.Kind != core.ConflictRename {
			t.Errorf("%s: Kind = %q, want rename", path, got.Kind)
		}
	}
}

func TestFR_CONF_07_ConflictKindSubmodule(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	dir := repo.Dir
	gittest.Commit(t, dir, "seed.txt", "seed\n", "seed")
	pointer := func(sha, msg string) {
		gittest.Run(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+sha+",sub")
		gittest.CommitAll(t, dir, msg)
	}
	pointer(strings.Repeat("1", 40), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	pointer(strings.Repeat("2", 40), "incoming")
	gittest.Run(t, dir, "checkout", "-q", "master")
	pointer(strings.Repeat("3", 40), "current")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "sub")

	if got.Kind != core.ConflictSubmodule {
		t.Errorf("Kind = %q", got.Kind)
	}
}

func TestFR_CONF_02_ConflictKindNotUTF8(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("caf\xe9 base\n"), version("caf\xe9 current\n"), version("caf\xe9 incoming\n"))

	got := readConflict(t, c, wt, "a.txt")

	if got.Kind != core.ConflictNotUTF8 {
		t.Errorf("Kind = %q", got.Kind)
	}
	if got.Current.Content != "" || got.Result != "" {
		t.Errorf("no content for a not-UTF-8 file: %+v", got)
	}
}

func TestFR_CONF_02_ConflictTextWithoutWorkingFileIsFileNotFound(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("base\n"), version("current\n"), version("incoming\n"))
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}

	_, err := c.Conflict(t.Context(), wt, "a.txt")

	_ = coretest.AsError(t, err, core.FileNotFound)
}

func TestFR_CONF_05_ConflictOnACleanPathIsNotConflicted(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("base\n"), version("current\n"), version("incoming\n"))

	_, err := c.Conflict(t.Context(), wt, "other.txt")

	requireGitFailedKey(t, err, "notConflicted")
}

func TestFR_CONF_05_ConflictLabelsTheSidesByOperation(t *testing.T) {
	t.Parallel()
	one, two, three := version("one\n"), version("two\n"), version("three\n")
	cases := []struct {
		name  string
		start func(t *testing.T) (dir string)
		want  func(sha func(rev string) string) (op core.Operation, current, incoming core.ConflictSide)
	}{
		{
			"merge",
			func(t *testing.T) string {
				t.Helper()
				dir := divergedRepo(t, "a.txt", one, two, three)
				tryGit(t, dir, "merge", "side")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return core.MergeOperation,
					core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"},
					core.ConflictSide{Present: true, SHA: sha("side"), Ref: "side"}
			},
		},
		{
			"rebase",
			func(t *testing.T) string {
				t.Helper()
				dir := divergedRepo(t, "a.txt", one, two, three)
				gittest.Run(t, dir, "checkout", "-q", "side")
				tryGit(t, dir, "rebase", "master")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return core.RebaseOperation,
					core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"},
					core.ConflictSide{Present: true, SHA: sha("side"), Subject: "incoming change"}
			},
		},
		{
			"rebase with the apply backend",
			func(t *testing.T) string {
				t.Helper()
				dir := divergedRepo(t, "a.txt", one, two, three)
				gittest.Run(t, dir, "checkout", "-q", "side")
				tryGit(t, dir, "-c", "rebase.backend=apply", "rebase", "master")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return core.RebaseOperation,
					core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"},
					core.ConflictSide{Present: true, SHA: sha("side"), Subject: "incoming change"}
			},
		},
		{
			"cherry-pick",
			func(t *testing.T) string {
				t.Helper()
				dir := divergedRepo(t, "a.txt", one, two, three)
				tryGit(t, dir, "cherry-pick", "side")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return core.CherryPickOperation,
					core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"},
					core.ConflictSide{Present: true, SHA: sha("side"), Subject: "incoming change"}
			},
		},
		{
			"revert",
			func(t *testing.T) string {
				t.Helper()
				dir := gittest.New(t).Dir
				commitVersion(t, dir, "a.txt", one, "base")
				commitVersion(t, dir, "a.txt", two, "reverted change")
				commitVersion(t, dir, "a.txt", three, "later change")
				gittest.Run(t, dir, "branch", "reverted", "HEAD~1")
				tryGit(t, dir, "revert", "HEAD~1")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return core.RevertOperation,
					core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"},
					core.ConflictSide{Present: true, SHA: sha("reverted"), Subject: "reverted change"}
			},
		},
		{
			"stash pop",
			func(t *testing.T) string {
				t.Helper()
				dir := gittest.New(t).Dir
				commitVersion(t, dir, "a.txt", one, "base")
				writeFile(t, dir, "a.txt", "two\n")
				gittest.Run(t, dir, "stash")
				commitVersion(t, dir, "a.txt", three, "later change")
				tryGit(t, dir, "stash", "pop")
				return dir
			},
			func(sha func(string) string) (core.Operation, core.ConflictSide, core.ConflictSide) {
				return "", core.ConflictSide{Present: true, SHA: sha("master"), Ref: "master"}, core.ConflictSide{Present: true}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.start(t)
			c, wt := openWorktree(t, dir)
			sha := func(rev string) string { return gittest.Out(t, dir, "rev-parse", rev) }

			got := readConflict(t, c, wt, "a.txt")

			op, current, incoming := tc.want(sha)
			if got.Operation != op {
				t.Errorf("Operation = %q, want %q", got.Operation, op)
			}
			current.Content, incoming.Content = got.Current.Content, got.Incoming.Content
			if got.Current != current || got.Incoming != incoming {
				t.Errorf("sides = %+v, %+v\nwant     %+v, %+v", got.Current, got.Incoming, current, incoming)
			}
		})
	}
}

func TestFR_CONF_05_ConflictRefPrefersLocalBranchThenRemoteThenTagFirstByName(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("one\n"), version("two\n"), version("three\n"))
	gittest.Run(t, dir, "tag", "v1", "side")
	gittest.Run(t, dir, "branch", "a-extra", "side")
	gittest.Run(t, dir, "update-ref", "refs/remotes/origin/aaa", "side")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	if got.Incoming.Ref != "a-extra" {
		t.Errorf("Incoming.Ref = %q, want a-extra", got.Incoming.Ref)
	}
}

func TestFR_CONF_05_ConflictRefFallsBackToRemoteBranchThenTag(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("one\n"), version("two\n"), version("three\n"))
	gittest.Run(t, dir, "tag", "v1", "side")
	gittest.Run(t, dir, "update-ref", "refs/remotes/origin/zzz", "side")
	gittest.Run(t, dir, "branch", "-D", "side")
	tryGit(t, dir, "merge", "origin/zzz")
	c, wt := openWorktree(t, dir)

	if got := readConflict(t, c, wt, "a.txt"); got.Incoming.Ref != "origin/zzz" {
		t.Errorf("Incoming.Ref = %q, want origin/zzz", got.Incoming.Ref)
	}
	gittest.Run(t, dir, "update-ref", "-d", "refs/remotes/origin/zzz")
	if got := readConflict(t, c, wt, "a.txt"); got.Incoming.Ref != "v1" {
		t.Errorf("Incoming.Ref = %q, want v1", got.Incoming.Ref)
	}
}
