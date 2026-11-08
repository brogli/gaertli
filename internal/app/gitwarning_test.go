// Requirements: QA-COMP-04

package app

import (
	"slices"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

var outdatedGit = core.Version{Git: "2.30.0", GitOutdated: true}

func TestTakeGitWarningWarnsOnceAboutAnOutdatedHostGit(t *testing.T) {
	r, _ := newTestRegistry()
	r.SetLocal(fakeAPI{version: outdatedGit})

	r.noteGit(t.Context(), "", r.Local())

	want := []GitWarning{{Version: "2.30.0", Latest: git.LatestCycle()}}
	if got := r.TakeGitWarnings(); !slices.Equal(got, want) {
		t.Fatalf("TakeGitWarnings() = %+v, want %+v", got, want)
	}
	if again := r.TakeGitWarnings(); len(again) != 0 {
		t.Errorf("second TakeGitWarnings() = %+v, want none", again)
	}
}

func TestTakeGitWarningHasNothingForACurrentGit(t *testing.T) {
	r, _ := newTestRegistry()
	r.SetLocal(fakeAPI{version: core.Version{Git: git.Latest}})

	r.noteGit(t.Context(), "", r.Local())

	if got := r.TakeGitWarnings(); len(got) != 0 {
		t.Errorf("TakeGitWarnings() = %+v, want none", got)
	}
}

func TestTakeGitWarningWarnsOnceAboutADistrosOutdatedGitOnFirstUse(t *testing.T) {
	r, _ := newTestRegistry()
	serveDistroCores(t, r, fakeAPI{version: outdatedGit})
	path := core.RepoPath(`\\wsl.localhost\Ubuntu-22.04\home\user\project`)

	if got := r.TakeGitWarnings(); len(got) != 0 {
		t.Fatalf("TakeGitWarnings() before first use = %+v, want none", got)
	}
	for range 2 {
		if _, err := r.CoreFor(path); err != nil {
			t.Fatal(err)
		}
	}

	want := []GitWarning{{Version: "2.30.0", Latest: git.LatestCycle(), Distro: "Ubuntu-22.04"}}
	if got := r.TakeGitWarnings(); !slices.Equal(got, want) {
		t.Fatalf("TakeGitWarnings() = %+v, want %+v", got, want)
	}
}

func TestTakeGitWarningsHasOneWarningPerOutdatedGit(t *testing.T) {
	r, _ := newTestRegistry()
	api := fakeAPI{version: outdatedGit}

	r.noteGit(t.Context(), "", api)
	r.noteGit(t.Context(), "Debian", api)
	r.noteGit(t.Context(), "Debian", api) // a reconnect

	if got := r.TakeGitWarnings(); len(got) != 2 || got[0].Distro != "" || got[1].Distro != "Debian" {
		t.Errorf("TakeGitWarnings() = %+v, want the host's, then Debian's", got)
	}
}
