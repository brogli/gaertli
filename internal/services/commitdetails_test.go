package services

import (
	"context"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_LOG_03_CommitDetailsReportsBodyParentsAndRename(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "a.txt", "x", "base")
	base := headSHA(t, repo.Dir)
	commit(t, repo.Dir, "a.txt", "y", "second\n\nmore detail\nsecond line")
	multiSHA := headSHA(t, repo.Dir)
	run(t, repo.Dir, "mv", "a.txt", "b.txt")
	run(t, repo.Dir, "commit", "-m", "rename")
	renameSHA := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	multi, err := c.CommitDetails(context.Background(), wt.ID, multiSHA)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(multi.Body, "more detail") || !strings.Contains(multi.Body, "second line") {
		t.Errorf("Body = %q, want the full multi-line message", multi.Body)
	}
	if len(multi.Parents) != 1 || multi.Parents[0] != base {
		t.Errorf("Parents = %v, want [%s]", multi.Parents, base)
	}
	if len(multi.Files) != 1 || multi.Files[0].Status != "M" {
		t.Errorf("Files = %+v, want one Modified file", multi.Files)
	}

	rename, err := c.CommitDetails(context.Background(), wt.ID, renameSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(rename.Files) != 1 || rename.Files[0].Status != "R" || rename.Files[0].OldPath != "a.txt" || rename.Files[0].Path != "b.txt" {
		t.Errorf("Files = %+v, want one Rename a.txt -> b.txt", rename.Files)
	}
}

func TestFR_LOG_03_CommitDetailsOnMergeDiffsAgainstFirstParentOnly(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "shared.txt", "base", "base")
	run(t, repo.Dir, "branch", "side")
	commit(t, repo.Dir, "main.txt", "m", "on main")
	run(t, repo.Dir, "switch", "side")
	commit(t, repo.Dir, "side.txt", "s", "on side")
	run(t, repo.Dir, "switch", "master")
	_, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: repo.Dir, Args: []string{"merge", "--no-ff", "-m", "merge side", "side"}})
	if err != nil {
		t.Fatal(err)
	}
	mergeSHA := headSHA(t, repo.Dir)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	details, err := c.CommitDetails(context.Background(), wt.ID, mergeSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Parents) != 2 {
		t.Fatalf("Parents = %v, want 2", details.Parents)
	}
	if len(details.Files) != 1 || details.Files[0].Path != "side.txt" {
		t.Errorf("Files = %+v, want exactly [side.txt] (the first-parent diff, not duplicated per parent)", details.Files)
	}
}

func TestFR_LOG_03_ParseCommitDetailsFieldsEmailsAndDates(t *testing.T) {
	header := strings.Join([]string{
		"aaaa", "bbbb cccc", "Alice", "alice@example.com", "1700000000",
		"Bob", "bob@example.com", "1700000100", "Subject line\n",
	}, "\x1f")
	stdout := []byte(header + "\x00\x00\nM\x00file.txt\x00")

	got, err := parseCommitDetails(stdout)
	if err != nil {
		t.Fatal(err)
	}
	want := core.CommitDetails{
		SHA: "aaaa", Parents: []string{"bbbb", "cccc"},
		Author: "Alice", AuthorEmail: "alice@example.com", AuthorAt: 1700000000,
		Committer: "Bob", CommitterEmail: "bob@example.com", CommitterAt: 1700000100,
		Body:  "Subject line",
		Files: []core.ChangedFile{{Status: "M", Path: "file.txt"}},
	}
	if got.SHA != want.SHA || got.Author != want.Author || got.AuthorEmail != want.AuthorEmail ||
		got.Committer != want.Committer || got.CommitterEmail != want.CommitterEmail ||
		got.AuthorAt != want.AuthorAt || got.CommitterAt != want.CommitterAt || got.Body != want.Body {
		t.Errorf("parseCommitDetails = %+v, want %+v", got, want)
	}
	if len(got.Files) != 1 || got.Files[0] != want.Files[0] {
		t.Errorf("Files = %+v, want %+v", got.Files, want.Files)
	}
}
