// Requirements: FR-DIFF-07

package app

import (
	"maps"
	"net/url"
	"slices"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestDiffWindowTitleNamesTheWorktreeAndWhatItShows(t *testing.T) {
	a := "1111111111111111111111111111111111111a"
	b := "2222222222222222222222222222222222222b"
	cases := []struct {
		target DiffTarget
		want   string
	}{
		{DiffTarget{Kind: CommitDiff, Sha: a}, "project: 1111111"},
		{DiffTarget{Kind: CommitDiff, Sha: "abc"}, "project: abc"},
		{DiffTarget{Kind: RangeDiff, From: a, To: b}, "project: compare 1111111..2222222"},
		{DiffTarget{Kind: AgainstWorkingDiff, Sha: a}, "project: compare 1111111..working tree"},
		{DiffTarget{Kind: WorkingDiff}, "project: changes"},
		{DiffTarget{Kind: WorkingDiff, Path: "a/b.txt", Conflicted: true}, "project: conflict in a/b.txt"},
	}
	for _, wt := range []core.WorktreeID{"/x/project", `\\wsl.localhost\Ubuntu\home\u\project`} {
		for _, c := range cases {
			if got := diffWindowTitle(wt, c.target); got != c.want {
				t.Errorf("diffWindowTitle(%q, %+v) = %q, want %q", wt, c.target, got, c.want)
			}
		}
	}
}

func TestDiffURLCarriesTheWorktreeAndTheTarget(t *testing.T) {
	base := url.Values{"view": {"diff"}, "wt": {"wt-1"}}
	with := func(fields url.Values) url.Values {
		q := maps.Clone(base)
		maps.Copy(q, fields)
		return q
	}
	cases := []struct {
		target DiffTarget
		want   url.Values
	}{
		{
			DiffTarget{Kind: CommitDiff, Sha: "s", Path: "f"},
			with(url.Values{"kind": {"commit"}, "sha": {"s"}, "file": {"f"}}),
		},
		{
			DiffTarget{Kind: RangeDiff, From: "a", To: "b", Path: "new", OldPath: "old"},
			with(url.Values{"kind": {"range"}, "from": {"a"}, "to": {"b"}, "file": {"new"}, "oldFile": {"old"}}),
		},
		{
			DiffTarget{Kind: AgainstWorkingDiff, Sha: "s", Path: "f"},
			with(url.Values{"kind": {"againstWorking"}, "sha": {"s"}, "file": {"f"}}),
		},
		{
			DiffTarget{Kind: WorkingDiff, Path: "f", Conflicted: true},
			with(url.Values{"kind": {"working"}, "conflicted": {"true"}, "file": {"f"}}),
		},
		{
			DiffTarget{Kind: WorkingDiff, Path: "f", Staged: true},
			with(url.Values{"kind": {"working"}, "staged": {"true"}, "file": {"f"}}),
		},
		{
			DiffTarget{Kind: WorkingDiff, Path: "f"},
			with(url.Values{"kind": {"working"}, "file": {"f"}}),
		},
	}
	for _, c := range cases {
		got := diffURL("wt-1", c.target)
		q, err := url.ParseQuery(got[len("/?"):])
		if err != nil {
			t.Fatalf("diffURL(%+v) = %q, not a query: %v", c.target, got, err)
		}
		if !maps.EqualFunc(q, c.want, slices.Equal) {
			t.Errorf("diffURL(%+v) query = %v, want %v", c.target, q, c.want)
		}
	}
}
