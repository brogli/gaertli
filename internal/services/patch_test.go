// Requirements: QA-COMP-01

package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestHunksOfReadsGitsU0HunkHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header, body string
		want         core.DiffHunk
	}{
		{"@@ -4,0 +5 @@", "+x\n", core.DiffHunk{OldStart: 4, OldLines: 0, NewStart: 5, NewLines: 1}},
		{"@@ -1 +0,0 @@", "-x\n", core.DiffHunk{OldStart: 1, OldLines: 1, NewStart: 0, NewLines: 0}},
		{"@@ -2 +2 @@", "-x\n+y\n", core.DiffHunk{OldStart: 2, OldLines: 1, NewStart: 2, NewLines: 1}},
		{"@@ -0,0 +1,3 @@", "+a\n+b\n+c\n", core.DiffHunk{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n" + tt.header + "\n" + tt.body

			file, err := parsePatch([]byte(patch), "f")
			if err != nil {
				t.Fatal(err)
			}

			got := hunksOf(file)
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("hunksOf = %+v, want [%+v]", got, tt.want)
			}
		})
	}
}

// writeScript writes an executable shell script running body and returns
// its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// framingFixture commits f.txt with a blank line 2 and edits lines 1 and
// 4: two -U0 hunks two lines apart, one -U3 hunk with a blank context line.
func framingFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "one\n\nthree\nfour\nfive\nsix\nseven\neight\n", "base")
	writeFile(t, dir, "f.txt", "ONE\n\nthree\nFOUR\nfive\nsix\nseven\neight\n")
}

func TestDiffWorkingAndStageLinesIgnoreConfigThatChangesThePatchFraming(t *testing.T) {
	t.Parallel()
	junk := writeScript(t, "echo EXTERNAL DIFF JUNK")
	configs := [][2]string{
		{"color.ui", "always"},
		{"diff.noprefix", "true"},
		{"diff.mnemonicPrefix", "true"},
		{"diff.external", junk},
		{"diff.suppressBlankEmpty", "true"},
		{"diff.interHunkContext", "3"},
	}
	original := gittest.New(t)
	framingFixture(t, original.Dir)
	for _, kv := range configs {
		t.Run(kv[0], func(t *testing.T) {
			repo := gittest.Copy(t, original.Dir)
			gittest.Run(t, repo.Dir, "config", kv[0], kv[1])

			got, err := diffWorking(t, repo.Dir, "f.txt", core.DiffOptions{})
			assertDiff(t, got, err,
				"one\n\nthree\nfour\nfive\nsix\nseven\neight\n", "ONE\n\nthree\nFOUR\nfive\nsix\nseven\neight\n",
				hunk(1, 1, 1, 1), hunk(4, 1, 4, 1))

			c, id := openWorktree(t, repo.Dir)
			sel := shown(t, c, id, "f.txt", false, []int{1}, []int{1})
			if err := c.StageLines(t.Context(), id, "f.txt", sel, false); err != nil {
				t.Fatal(err)
			}

			want := "ONE\n\nthree\nfour\nfive\nsix\nseven\neight"
			if got := indexBlob(t, repo.Dir, "f.txt"); got != want {
				t.Errorf("index f.txt = %q, want %q (only the first hunk staged)", got, want)
			}
		})
	}
}

// modeFixture commits f.txt, then makes it executable and edits line 2.
func modeFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "a\nb\nc\n", "base")
	if err := os.Chmod(filepath.Join(dir, "f.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "f.txt", "a\nB\nc\n")
}

func TestStageAndDiscardLinesLeaveTheFileModeAlone(t *testing.T) {
	t.Parallel()
	t.Run("stage", func(t *testing.T) {
		repo := gittest.New(t)
		modeFixture(t, repo.Dir)
		c, id := openWorktree(t, repo.Dir)

		if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2}), false); err != nil {
			t.Fatal(err)
		}

		if got := gittest.Out(t, repo.Dir, "ls-files", "-s", "--", "f.txt"); !strings.HasPrefix(got, "100644 ") {
			t.Errorf("index entry = %q, want mode 100644 kept", got)
		}
		if got := indexBlob(t, repo.Dir, "f.txt"); got != "a\nB\nc" {
			t.Errorf("index f.txt = %q, want line 2 staged", got)
		}
	})

	t.Run("discard", func(t *testing.T) {
		repo := gittest.New(t)
		modeFixture(t, repo.Dir)
		c, id := openWorktree(t, repo.Dir)

		if err := c.DiscardLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2})); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(filepath.Join(repo.Dir, "f.txt"))
		if err != nil {
			t.Fatal(err)
		}
		t.Run("mode", func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows files have no executable bit")
			}
			if info.Mode().Perm() != 0o755 {
				t.Errorf("f.txt mode = %v, want 0755 kept", info.Mode().Perm())
			}
		})
		if got := readFile(t, repo.Dir, "f.txt"); got != "a\nb\nc\n" {
			t.Errorf("f.txt = %q, want line 2 discarded", got)
		}
	})
}

// textconvFixture makes git diff show *.txt files upper-cased and edits
// line 2 of a lower-case f.txt.
func textconvFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Run(t, dir, "config", "diff.up.textconv", writeScript(t, `tr a-z A-Z < "$1"`))
	gittest.Commit(t, dir, ".gitattributes", "*.txt diff=up\n", "attributes")
	gittest.Commit(t, dir, "f.txt", "hello\nworld\n", "base")
	writeFile(t, dir, "f.txt", "hello\nthere\n")
}

func TestDiffWorkingAndStageLinesUseTheRealTextNotTheTextconvOutput(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	textconvFixture(t, repo.Dir)

	got, err := diffWorking(t, repo.Dir, "f.txt", core.DiffOptions{})
	assertDiff(t, got, err, "hello\nworld\n", "hello\nthere\n", hunk(2, 1, 2, 1))

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.StageLines(t.Context(), id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	if got := indexBlob(t, repo.Dir, "f.txt"); got != "hello\nthere" {
		t.Errorf("index f.txt = %q, want %q", got, "hello\nthere")
	}
}
