// Requirements: DIST-03, QA-COMP-04

package services

import (
	"testing"

	"github.com/brogli/gaertli/internal/buildinfo"
	"github.com/brogli/gaertli/internal/git"
)

func TestVersionReturnsAppAndCommit(t *testing.T) {
	origApp, origCommit := buildinfo.App, buildinfo.Commit
	buildinfo.App, buildinfo.Commit = "0.1.57", "abc1234"
	t.Cleanup(func() { buildinfo.App, buildinfo.Commit = origApp, origCommit })

	v, err := New(nil).Version(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if v.App != "0.1.57" || v.Commit != "abc1234" {
		t.Errorf("Version = %+v, want App=0.1.57 Commit=abc1234", v)
	}
}

func TestVersionReportsAnOutdatedGit(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	c := New(nil)
	c.discover = git.Runner{Git: fakegit}

	v, err := c.Version(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if v.Git != "2.30.0" || !v.GitOutdated {
		t.Errorf("Version = %+v, want Git=2.30.0 GitOutdated", v)
	}
}

func TestVersionReportsAMissingGitAsUnknownNotOutdated(t *testing.T) {
	c := New(nil)
	c.discover = git.Runner{Git: "/does/not/exist"}

	v, err := c.Version(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if v.Git != "" || v.GitOutdated {
		t.Errorf("Version = %+v, want Git empty, not outdated", v)
	}
}
