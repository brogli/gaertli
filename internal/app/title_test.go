package app

import (
	"net/url"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestFR_WT_09_WindowTitle(t *testing.T) {
	cases := []struct{ path, branch, want string }{
		{"", "", "nebegit"},
		{"/x/project", "", "project"},
		{"/x/project", "main", "project [main]"},
	}
	for _, c := range cases {
		if got := windowTitle(c.path, c.branch); got != c.want {
			t.Errorf("windowTitle(%q, %q) = %q, want %q", c.path, c.branch, got, c.want)
		}
	}
}

func TestFR_CMP_02_CompareWindowTitle(t *testing.T) {
	a := "1111111111111111111111111111111111111a"
	b := "2222222222222222222222222222222222222b"

	got := compareWindowTitle("/x/project", a, b)

	if want := "project: compare 1111111..2222222"; got != want {
		t.Errorf("compareWindowTitle(...) = %q, want %q", got, want)
	}
}

func TestFR_CMP_02_ShortSHA(t *testing.T) {
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA(%q) = %q, want %q", "abc", got, "abc")
	}
}

func TestFR_CMP_02_CompareDiffURL(t *testing.T) {
	got := compareDiffURL(core.WorktreeID("wt-1"), "a", "b", "f1.txt", "")
	q, err := url.ParseQuery(got[len("/?"):])
	if err != nil {
		t.Fatalf("compareDiffURL produced an unparseable query: %v", err)
	}
	if q.Get("view") != "compare-diff" || q.Get("wt") != "wt-1" || q.Get("from") != "a" ||
		q.Get("to") != "b" || q.Get("file") != "f1.txt" {
		t.Errorf("compareDiffURL(...) query = %v, missing an expected field", q)
	}
	if q.Has("oldFile") {
		t.Errorf("compareDiffURL(...) query has oldFile = %q, want absent", q.Get("oldFile"))
	}

	got = compareDiffURL(core.WorktreeID("wt-1"), "a", "b", "f1.txt", "old.txt")
	q, err = url.ParseQuery(got[len("/?"):])
	if err != nil {
		t.Fatalf("compareDiffURL produced an unparseable query: %v", err)
	}
	if q.Get("oldFile") != "old.txt" {
		t.Errorf("compareDiffURL(...) query oldFile = %q, want %q", q.Get("oldFile"), "old.txt")
	}
}
