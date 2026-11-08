// Requirements: DIST-01, QA-COMP-04

package app

import (
	"testing"

	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/testbin"
)

// fakegit is internal/git/testdata/fakegit, built once for every test in
// this package.
var fakegit string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &fakegit, Pkg: "github.com/brogli/gaertli/internal/git/testdata/fakegit"})
}

func TestRefusesAMissingGit(t *testing.T) {
	if gitRuns(t.Context(), git.Runner{Git: "/does/not/exist"}) {
		t.Error("gitRuns() = true for a missing git")
	}
}

func TestAcceptsAnOldGit(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	if !gitRuns(t.Context(), git.Runner{Git: fakegit}) {
		t.Error("gitRuns() = false for git 2.30.0")
	}
}

func TestAcceptsInstalledGit(t *testing.T) {
	if !gitRuns(t.Context(), git.Runner{}) {
		t.Error("gitRuns() = false with the real, installed git")
	}
}
