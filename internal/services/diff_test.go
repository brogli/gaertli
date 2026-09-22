package services

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestADR_7_ParseCatFileBatchSplitsFoundAndMissingObjectsInOrder(t *testing.T) {
	data := []byte("aaaa111 blob 5\nhello\nsha^:missing.txt missing\nbbbb222 blob 3\nbye\n")

	blobs, err := parseCatFileBatch(data, 3)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]byte{[]byte("hello"), nil, []byte("bye")}
	for i := range want {
		if !bytes.Equal(blobs[i], want[i]) {
			t.Errorf("blobs[%d] = %q, want %q", i, blobs[i], want[i])
		}
	}
}

func TestADR_7_BlobSizeParsesANumberOrReadsMissingAsZero(t *testing.T) {
	size, err := blobSize("12345")
	if err != nil || size != 12345 {
		t.Errorf("blobSize(number) = %d, %v, want 12345, nil", size, err)
	}

	size, err = blobSize("sha^:missing.txt missing")
	if err != nil || size != 0 {
		t.Errorf("blobSize(missing) = %d, %v, want 0, nil", size, err)
	}
}

func TestFR_DIFF_03_DiffIsBinaryClassifiesNumstatRecords(t *testing.T) {
	// A fake git.Runner's stdout goes through an OS environment variable
	// (FAKEGIT_STDOUT), which cannot carry an embedded NUL byte — so the
	// rename fixture (whose real -z record embeds one, Research used) is
	// exercised directly against parseBinaryNumstat below instead.
	tests := []struct {
		name       string
		stdout     string
		wantBinary bool
	}{
		{"binary file", "-\t-\tfile", true},
		{"text file", "3\t1\tfile", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FAKEGIT_STDOUT", tt.stdout)
			r := git.Runner{Git: fakegit}

			got, err := diffIsBinary(context.Background(), r, t.TempDir(), "sha", "", "file")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantBinary {
				t.Errorf("diffIsBinary = %v, want %v", got, tt.wantBinary)
			}
		})
	}
}

func TestFR_DIFF_03_ParseBinaryNumstatIgnoresARenameRecordsThirdField(t *testing.T) {
	record := []byte("1\t0\t\x00old\x00new\x00")

	if parseBinaryNumstat(record) {
		t.Error("parseBinaryNumstat(rename record) = true, want false")
	}
}

func TestFR_LOG_04_DiffFileReturnsExactContentOnBothSides(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "a.txt", "first\n", "first")
	commit(t, repo.Dir, "a.txt", "second\n", "second")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "first\n" || diff.NewContent != "second\n" {
		t.Errorf("DiffFile = %+v, want OldContent %q NewContent %q", diff, "first\n", "second\n")
	}
}

func TestFR_LOG_04_DiffFileOnAddedFileHasNoOldContent(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "a.txt", "added text\n", "add a.txt")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "added text\n" {
		t.Errorf("DiffFile = %+v, want OldContent \"\" NewContent %q", diff, "added text\n")
	}
}

func TestFR_LOG_04_DiffFileOnDeletedFileHasNoNewContent(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "a.txt", "still here\n", "add a.txt")
	run(t, repo.Dir, "rm", "a.txt")
	run(t, repo.Dir, "commit", "-m", "remove a.txt")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "still here\n" || diff.NewContent != "" {
		t.Errorf("DiffFile = %+v, want OldContent %q NewContent \"\"", diff, "still here\n")
	}
}

func TestFR_LOG_04_DiffFileOnRenameWithContentChangeUsesOldPath(t *testing.T) {
	repo := gittest.New(t)
	base := strings.Repeat("line unchanged\n", 20)
	commit(t, repo.Dir, "a.txt", base+"old ending\n", "add a.txt")
	renameWithEdit(t, repo.Dir, "a.txt", "b.txt", "new ending") // compare_test.go
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "b.txt", "a.txt", false)
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
	repo := gittest.New(t)
	commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "bin.dat", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary || diff.OldContent != "" || diff.NewContent != "" {
		t.Errorf("DiffFile = %+v, want Binary with no content", diff)
	}
}

func TestFR_DIFF_03_DiffFileTreatsGitattributesDiffOverrideAsBinary(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, ".gitattributes", "a.txt -diff\n", "add gitattributes")
	commit(t, repo.Dir, "a.txt", "plain text, no nul bytes\n", "add a.txt")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary {
		t.Errorf("DiffFile.Binary = false, want true (a plain-text file marked -diff)")
	}
}

func TestFR_DIFF_04_DiffFileCapsHugeFilesWithAForceReloadBypass(t *testing.T) {
	repo := gittest.New(t)
	big := strings.Repeat("a", maxDiffFileSize+1)
	commit(t, repo.Dir, "big.txt", big, "add big file")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	capped, err := c.DiffFile(context.Background(), wt.ID, sha, "big.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !capped.TooLarge || capped.OldContent != "" || capped.NewContent != "" {
		t.Errorf("DiffFile = %+v, want TooLarge with no content", capped)
	}

	full, err := c.DiffFile(context.Background(), wt.ID, sha, "big.txt", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if full.TooLarge || full.NewContent != big {
		t.Error("DiffFile with force=true: still TooLarge or content mismatch, want the full content")
	}
}

func TestQA_PERF_05_DiffFileUnder1MBCompletesWithinBudget(t *testing.T) {
	repo := gittest.New(t)
	content := strings.Repeat("a line of committed text\n", 700*1024/25) // ~700KB, under maxDiffFileSize
	commit(t, repo.Dir, "file.txt", content, "add a ~700KB file")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "file.txt", "", false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if diff.TooLarge || diff.NewContent != content {
		t.Fatal("DiffFile did not return the full content")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("DiffFile took %s, want <= 500ms", elapsed)
	}
}

func TestADR_7_DiffFileOnRootCommitHasNoOldContent(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "a.txt", "root content\n", "root commit")
	sha := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	diff, err := c.DiffFile(context.Background(), wt.ID, sha, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "root content\n" {
		t.Errorf("DiffFile = %+v, want OldContent \"\" NewContent %q", diff, "root content\n")
	}
}
