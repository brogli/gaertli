package gittest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTR_14_CopyIsAnIndependentRepoWithTheSameState(t *testing.T) {
	original := New(t)
	shas := History(t, original.Dir, Step{Message: "base", Files: map[string]string{"a.txt": "a\n"}})
	if err := os.WriteFile(filepath.Join(original.Dir, "a.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cp := Copy(t, original.Dir)
	Commit(t, cp.Dir, "b.txt", "b\n", "only in the copy")

	if got := Out(t, cp.Dir, "log", "--format=%s", "-2"); got != "only in the copy\nbase" {
		t.Errorf("copy log = %q", got)
	}
	if got := Out(t, cp.Dir, "status", "--porcelain"); got != "M a.txt" {
		t.Errorf("copy status = %q, want the edit carried over", got)
	}
	if got := HeadSHA(t, original.Dir); got != shas[0] {
		t.Errorf("original HEAD = %s, want unchanged %s", got, shas[0])
	}
}

func TestTR_14_HistoryWritesBranchesAndMergeInOneRun(t *testing.T) {
	repo := New(t)

	shas := History(t, repo.Dir,
		Step{Message: "base", Files: map[string]string{"a.txt": "a\n", "gone.txt": "g\n"}},
		Step{Branch: "side", From: "master", Message: "side change\n\nbody", Author: "Bea", Files: map[string]string{"s.txt": "s\n"}},
		Step{Message: "master change", Files: map[string]string{"m.txt": "m\n"}, Delete: []string{"gone.txt"}},
		Step{Message: "merge", Merge: "side", Files: map[string]string{"s.txt": "s\n"}},
	)

	if len(shas) != 4 {
		t.Fatalf("got %d SHAs, want 4", len(shas))
	}
	if got := HeadSHA(t, repo.Dir); got != shas[3] {
		t.Errorf("HEAD = %s, want the merge %s", got, shas[3])
	}
	if got := Out(t, repo.Dir, "rev-parse", "side"); got != shas[1] {
		t.Errorf("side = %s, want %s", got, shas[1])
	}
	if got := Out(t, repo.Dir, "rev-list", "--parents", "-1", "HEAD"); got != shas[3]+" "+shas[2]+" "+shas[1] {
		t.Errorf("merge parents = %q", got)
	}
	if got := Out(t, repo.Dir, "ls-files"); got != "a.txt\nm.txt\ns.txt" {
		t.Errorf("index = %q, want the merged files without the deleted one", got)
	}
	if got := Out(t, repo.Dir, "status", "--porcelain"); got != "" {
		t.Errorf("working tree not clean: %q", got)
	}
	want := "Bea <bea@nebegit.invalid> / nebegit test <test@nebegit.invalid> / side change\n\nbody"
	if got := Out(t, repo.Dir, "log", "-1", "--format=%an <%ae> / %cn <%ce> / %B", shas[1]); got != want {
		t.Errorf("identity and message = %q, want %q", got, want)
	}
}
