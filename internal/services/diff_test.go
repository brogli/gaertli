package services

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func hunk(oldStart, oldLines, newStart, newLines int) core.DiffHunk {
	return core.DiffHunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines}
}

// assertDiff checks that a Diff* call returned a text diff with these
// contents and hunks.
func assertDiff(t *testing.T, got core.DiffFile, err error, oldContent, newContent string, hunks ...core.DiffHunk) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if got.Binary || got.TooLarge || got.OldContent != oldContent || got.NewContent != newContent || !slices.Equal(got.Hunks, hunks) {
		t.Errorf("diff = %+v\nwant OldContent %q NewContent %q Hunks %v", got, oldContent, newContent, hunks)
	}
}

// diffWorking is DiffWorking of path's unstaged diff on a fresh core.
func diffWorking(t *testing.T, dir, path string, opts core.DiffOptions) (core.DiffFile, error) {
	t.Helper()
	c, id := openWorktree(t, dir)
	return c.DiffWorking(t.Context(), id, path, "", false, opts)
}

// movedLineFixture commits m.txt "A B C A B C D", one per line, and moves
// the D up in the working tree: myers and histogram read the move
// differently.
func movedLineFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "m.txt", "A\nB\nC\nA\nB\nC\nD\n", "base")
	writeFile(t, dir, "m.txt", "A\nB\nC\nD\nA\nB\nC\n")
}

func TestQA_COMP_01_DiffWorkingHunksFollowTheConfiguredDiffAlgorithm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		algorithm string
		want      []core.DiffHunk
	}{
		{"myers", []core.DiffHunk{hunk(3, 0, 4, 1), hunk(7, 1, 7, 0)}},
		{"histogram", []core.DiffHunk{hunk(4, 3, 3, 0), hunk(7, 0, 5, 3)}},
	}
	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			repo := gittest.New(t)
			movedLineFixture(t, repo.Dir)
			gittest.Run(t, repo.Dir, "config", "diff.algorithm", tt.algorithm)

			got, err := diffWorking(t, repo.Dir, "m.txt", core.DiffOptions{})

			assertDiff(t, got, err, "A\nB\nC\nA\nB\nC\nD\n", "A\nB\nC\nD\nA\nB\nC\n", tt.want...)
		})
	}
}

// autocrlfFixture commits a 3-line f.txt under core.autocrlf=true (stored
// with LF) and leaves the working file CRLF with line 2 changed.
func autocrlfFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Run(t, dir, "config", "core.autocrlf", "true")
	gittest.Commit(t, dir, "f.txt", "l1\r\nl2\r\nl3\r\n", "base")
	writeFile(t, dir, "f.txt", "l1\r\nCHANGED\r\nl3\r\n")
}

func TestQA_COMP_01_DiffWorkingShowsACRLFWorkingFileInTheCleanForm(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	autocrlfFixture(t, repo.Dir)

	got, err := diffWorking(t, repo.Dir, "f.txt", core.DiffOptions{})

	assertDiff(t, got, err, "l1\nl2\nl3\n", "l1\nCHANGED\nl3\n", hunk(2, 1, 2, 1))
}

// filterFixture commits f through a clean filter that upper-cases it and
// leaves the working file lower-case with line 2 changed.
func filterFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Run(t, dir, "config", "filter.up.clean", "tr a-z A-Z")
	gittest.Run(t, dir, "config", "filter.up.smudge", "cat")
	gittest.Commit(t, dir, ".gitattributes", "f filter=up\n", "attributes")
	gittest.Commit(t, dir, "f", "HELLO\nWORLD\n", "base")
	writeFile(t, dir, "f", "hello\nthere\n")
}

func TestQA_COMP_01_DiffWorkingShowsAFilteredWorkingFileInTheCleanForm(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	filterFixture(t, repo.Dir)

	got, err := diffWorking(t, repo.Dir, "f", core.DiffOptions{})

	assertDiff(t, got, err, "HELLO\nWORLD\n", "HELLO\nTHERE\n", hunk(2, 1, 2, 1))
}

// whitespaceFixture commits f.txt and edits line 2 by whitespace only and
// line 3 for real.
func whitespaceFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "one\ntwo\nthree\n", "base")
	writeFile(t, dir, "f.txt", "one\ntwo  \nTHREE\n")
}

func TestFR_DIFF_06_DiffWorkingIgnoringWhitespaceKeepsOnlyRealChangesInTheHunks(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	whitespaceFixture(t, repo.Dir)

	full, fullErr := diffWorking(t, repo.Dir, "f.txt", core.DiffOptions{})
	ignoring, ignoringErr := diffWorking(t, repo.Dir, "f.txt", core.DiffOptions{IgnoreWhitespace: true})

	assertDiff(t, full, fullErr, "one\ntwo\nthree\n", "one\ntwo  \nTHREE\n", hunk(2, 2, 2, 2))
	assertDiff(t, ignoring, ignoringErr, "one\ntwo\nthree\n", "one\ntwo  \nTHREE\n", hunk(3, 1, 3, 1))
}

func TestQA_COMP_01_DiffCallsReturnGitsContentsAndHunksForEveryKindOfChange(t *testing.T) {
	t.Parallel()
	none := core.DiffOptions{}
	body := strings.Repeat("line unchanged\n", 20)

	t.Run("root commit", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "a.txt", "root\n", "root")

		got, err := c.DiffFile(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "a.txt", "", none)

		assertDiff(t, got, err, "", "root\n", hunk(0, 0, 1, 1))
	})

	t.Run("merge against its first parent", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "b.txt", "b\n", "base")
		gittest.Run(t, repo.Dir, "checkout", "-q", "-b", "side")
		gittest.Commit(t, repo.Dir, "b.txt", "b side\n", "side")
		gittest.Run(t, repo.Dir, "checkout", "-q", "-")
		gittest.Commit(t, repo.Dir, "a.txt", "a\n", "main")
		gittest.Run(t, repo.Dir, "merge", "-q", "--no-ff", "-m", "merge", "side")

		got, err := c.DiffFile(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "b.txt", "", none)

		assertDiff(t, got, err, "b\n", "b side\n", hunk(1, 1, 1, 1))
	})

	t.Run("rename with an edit", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "a.txt", body+"old ending\n", "add a.txt")
		renameWithEdit(t, repo.Dir, "a.txt", "b.txt", "new ending")

		got, err := c.DiffFile(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "b.txt", "a.txt", none)

		assertDiff(t, got, err, body+"old ending\n", body+"new ending\n", hunk(21, 1, 21, 1))
	})

	t.Run("two revisions", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "f", "1\n2\n3\n", "a")
		a := gittest.HeadSHA(t, repo.Dir)
		gittest.Commit(t, repo.Dir, "f", "1\nTWO\n3\n", "b")

		got, err := c.DiffRange(t.Context(), id, a, gittest.HeadSHA(t, repo.Dir), "f", "", none)

		assertDiff(t, got, err, "1\n2\n3\n", "1\nTWO\n3\n", hunk(2, 1, 2, 1))
	})

	t.Run("added", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "x", "x\n", "base")
		writeFile(t, repo.Dir, "new.txt", "n1\nn2\n")
		gittest.Run(t, repo.Dir, "add", "new.txt")

		got, err := c.DiffWorking(t.Context(), id, "new.txt", "", true, none)

		assertDiff(t, got, err, "", "n1\nn2\n", hunk(0, 0, 1, 2))
	})

	t.Run("deleted", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "a.txt", "a1\na2\n", "base")
		if err := os.Remove(filepath.Join(repo.Dir, "a.txt")); err != nil {
			t.Fatal(err)
		}

		got, err := c.DiffWorking(t.Context(), id, "a.txt", "", false, none)

		assertDiff(t, got, err, "a1\na2\n", "", hunk(1, 2, 0, 0))
	})

	t.Run("untracked", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "x", "x\n", "base")
		writeFile(t, repo.Dir, "u.txt", "u\n")

		got, err := c.DiffWorking(t.Context(), id, "u.txt", "", false, none)

		assertDiff(t, got, err, "", "u\n", hunk(0, 0, 1, 1))
	})

	t.Run("a commit against the working tree", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "f", "1\n2\n", "base")
		writeFile(t, repo.Dir, "f", "1\n2\n3\n")

		got, err := c.DiffAgainstWorking(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "f", "", none)

		assertDiff(t, got, err, "1\n2\n", "1\n2\n3\n", hunk(2, 0, 3, 1))
	})

	t.Run("a commit against an untracked file", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "x", "x\n", "base")
		writeFile(t, repo.Dir, "u.txt", "u\n")

		got, err := c.DiffAgainstWorking(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "u.txt", "", none)

		assertDiff(t, got, err, "", "u\n", hunk(0, 0, 1, 1))
	})

	t.Run("pure rename", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "a.txt", "same\n", "base")
		gittest.Run(t, repo.Dir, "mv", "a.txt", "b.txt")

		got, err := c.DiffWorking(t.Context(), id, "b.txt", "a.txt", true, none)

		assertDiff(t, got, err, "same\n", "same\n")
	})

	t.Run("mode-only change", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "a.txt", "same\n", "base")
		if err := os.Chmod(filepath.Join(repo.Dir, "a.txt"), 0o755); err != nil {
			t.Fatal(err)
		}

		got, err := c.DiffWorking(t.Context(), id, "a.txt", "", false, none)

		assertDiff(t, got, err, "same\n", "same\n")
	})

	t.Run("last line without a newline", func(t *testing.T) {
		repo, c, id := openCompareRepo(t)
		gittest.Commit(t, repo.Dir, "f", "a\nb", "base")
		writeFile(t, repo.Dir, "f", "a\nB")

		got, err := c.DiffWorking(t.Context(), id, "f", "", false, none)

		assertDiff(t, got, err, "a\nb", "a\nB", hunk(2, 1, 2, 1))
	})
}

func TestFR_DIFF_03_DiffCallsTreatFilesGitCallsBinaryAsBinary(t *testing.T) {
	t.Parallel()
	for _, attributes := range []string{"*.dat binary\n", "*.dat -diff\n"} {
		t.Run(attributes, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, ".gitattributes", attributes, "attributes")
			gittest.Commit(t, repo.Dir, "t.dat", "plain text\n", "add t.dat")
			writeFile(t, repo.Dir, "t.dat", "changed text\n")
			c, id := openWorktree(t, repo.Dir)

			committed, err := c.DiffFile(t.Context(), id, gittest.HeadSHA(t, repo.Dir), "t.dat", "", core.DiffOptions{})
			if err != nil {
				t.Fatal(err)
			}
			working, err := c.DiffWorking(t.Context(), id, "t.dat", "", false, core.DiffOptions{})
			if err != nil {
				t.Fatal(err)
			}

			for _, got := range []core.DiffFile{committed, working} {
				if !got.Binary || got.OldContent != "" || got.NewContent != "" || got.Hunks != nil {
					t.Errorf("diff = %+v, want Binary with no content and no hunks", got)
				}
			}
		})
	}
}

func TestADR_7_ParseCatFileBatchReadsAFoundOrAMissingObject(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		data string
		want []byte
	}{
		{"aaaa111 blob 6\x00he\x00llo\x00", []byte("he\x00llo")},
		{"sha^:missing.txt missing\x00", nil},
	} {
		got, err := parseCatFileBatch([]byte(tc.data))
		if err != nil || !bytes.Equal(got, tc.want) {
			t.Errorf("parseCatFileBatch(%q) = %q, %v; want %q", tc.data, got, err, tc.want)
		}
	}
}

func TestADR_7_BlobSizeParsesANumberOrReadsMissingAsZero(t *testing.T) {
	t.Parallel()
	size, err := blobSize("12345")
	if err != nil || size != 12345 {
		t.Errorf("blobSize(number) = %d, %v, want 12345, nil", size, err)
	}

	size, err = blobSize("sha^:missing.txt missing")
	if err != nil || size != 0 {
		t.Errorf("blobSize(missing) = %d, %v, want 0, nil", size, err)
	}
}

func TestFR_LOG_04_DiffFileReturnsExactContentOnBothSides(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "first\n", "first")
	gittest.Commit(t, repo.Dir, "a.txt", "second\n", "second")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "a.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "first\n" || diff.NewContent != "second\n" {
		t.Errorf("DiffFile = %+v, want OldContent %q NewContent %q", diff, "first\n", "second\n")
	}
}

func TestFR_LOG_04_DiffFileOnAddedFileHasNoOldContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "added text\n", "add a.txt")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "a.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "added text\n" {
		t.Errorf("DiffFile = %+v, want OldContent \"\" NewContent %q", diff, "added text\n")
	}
}

func TestFR_LOG_04_DiffFileOnDeletedFileHasNoNewContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "still here\n", "add a.txt")
	gittest.Run(t, repo.Dir, "rm", "a.txt")
	gittest.Run(t, repo.Dir, "commit", "-m", "remove a.txt")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "a.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "still here\n" || diff.NewContent != "" {
		t.Errorf("DiffFile = %+v, want OldContent %q NewContent \"\"", diff, "still here\n")
	}
}

func TestFR_LOG_04_DiffFileOnRenameWithContentChangeUsesOldPath(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := strings.Repeat("line unchanged\n", 20)
	gittest.Commit(t, repo.Dir, "a.txt", base+"old ending\n", "add a.txt")
	renameWithEdit(t, repo.Dir, "a.txt", "b.txt", "new ending")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "b.txt", "a.txt", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldPath != "a.txt" {
		t.Errorf("OldPath = %q, want a.txt", diff.OldPath)
	}
	if diff.OldContent != base+"old ending\n" || diff.NewContent != base+"new ending\n" {
		t.Errorf("DiffFile content mismatch:\nold = %q\nnew = %q", diff.OldContent, diff.NewContent)
	}
}

func TestFR_DIFF_03_DiffFileOnEmbeddedNULByteIsBinaryWithNoContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "bin.dat", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary || diff.OldContent != "" || diff.NewContent != "" {
		t.Errorf("DiffFile = %+v, want Binary with no content", diff)
	}
}

func TestFR_DIFF_03_DiffFileTreatsGitattributesDiffOverrideAsBinary(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, ".gitattributes", "a.txt -diff\n", "add gitattributes")
	gittest.Commit(t, repo.Dir, "a.txt", "plain text, no nul bytes\n", "add a.txt")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "a.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary {
		t.Errorf("DiffFile.Binary = false, want true (a plain-text file marked -diff)")
	}
}

func TestFR_DIFF_04_DiffFileCapsHugeFilesWithAForceReloadBypass(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	big := strings.Repeat("a", maxDiffFileSize+1)
	gittest.Commit(t, repo.Dir, "big.txt", big, "add big file")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	capped, err := c.DiffFile(t.Context(), wt, sha, "big.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !capped.TooLarge || capped.OldContent != "" || capped.NewContent != "" {
		t.Errorf("DiffFile = %+v, want TooLarge with no content", capped)
	}

	full, err := c.DiffFile(t.Context(), wt, sha, "big.txt", "", core.DiffOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if full.TooLarge || full.NewContent != big {
		t.Error("DiffFile with force=true: still TooLarge or content mismatch, want the full content")
	}
	if want := []core.DiffHunk{hunk(0, 0, 1, 1)}; !slices.Equal(full.Hunks, want) {
		t.Errorf("DiffFile with force=true: Hunks = %v, want %v", full.Hunks, want)
	}
}

func TestFR_DIFF_04_DiffFileUnder1MBReturnsTheFullContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	content := strings.Repeat("a line of committed text\n", 700*1024/25) // ~700KB, under maxDiffFileSize
	gittest.Commit(t, repo.Dir, "file.txt", content, "add a ~700KB file")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "file.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.TooLarge || diff.NewContent != content {
		t.Fatal("DiffFile did not return the full content")
	}
}

func TestADR_7_DiffFileOnRootCommitHasNoOldContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "root content\n", "root commit")
	sha := gittest.HeadSHA(t, repo.Dir)

	c, wt := openWorktree(t, repo.Dir)

	diff, err := c.DiffFile(t.Context(), wt, sha, "a.txt", "", core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "root content\n" {
		t.Errorf("DiffFile = %+v, want OldContent \"\" NewContent %q", diff, "root content\n")
	}
}
