package app

import (
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/testbin"
)

// fakegit is internal/git/testdata/fakegit, built once for every test in
// this package.
var fakegit string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &fakegit, Pkg: "github.com/brogli/nebegit/internal/git/testdata/fakegit"})
}

func TestDIST_01_ChecksGitPresence(t *testing.T) {
	ok, msg := checkGit(t.Context(), git.Runner{Git: "/does/not/exist"})
	if ok {
		t.Fatal("checkGit() ok = true for a missing git")
	}
	if !strings.Contains(msg, "git") {
		t.Errorf("msg = %q, want it to mention git", msg)
	}
}

func TestDIST_01_ChecksGitVersion(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")

	ok, msg := checkGit(t.Context(), git.Runner{Git: fakegit})

	if ok {
		t.Fatal("checkGit() ok = true for git 2.30.0")
	}
	for _, want := range []string{"2.30.0", git.MinVersion.String()} {
		if !strings.Contains(msg, want) {
			t.Errorf("msg = %q, want it to contain %q", msg, want)
		}
	}
}

func TestDIST_01_AcceptsInstalledGit(t *testing.T) {
	ok, msg := checkGit(t.Context(), git.Runner{})
	if !ok {
		t.Fatalf("checkGit() ok = false with the real, installed git; msg = %q", msg)
	}
}

func TestDIST_01_Messages(t *testing.T) {
	msg := text(startupFailed, map[string]string{"error": "x"})
	for _, want := range []string{"WebView2", "webkitgtk", "x"} {
		if !strings.Contains(msg, want) {
			t.Errorf("startupFailed = %q, want it to contain %q", msg, want)
		}
	}
}
