// Requirements: QA-COMP-04

package git

import (
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
)

func TestCheckVersionAcceptsInstalledGit(t *testing.T) {
	v, err := (Runner{}).CheckVersion(t.Context())
	if err != nil {
		t.Fatalf("CheckVersion() err = %v, want nil", err)
	}
	if v == (Version{}) {
		t.Error("CheckVersion() = zero Version, want the installed git's")
	}
}

func TestCheckVersionAcceptsOldGit(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	v, err := (Runner{Git: fakegit}).CheckVersion(t.Context())
	if err != nil {
		t.Fatalf("CheckVersion() err = %v, want nil", err)
	}
	if v != (Version{Major: 2, Minor: 30, Patch: 0}) || !v.Outdated() {
		t.Errorf("CheckVersion() = %v, outdated %v; want 2.30.0, outdated", v, v.Outdated())
	}
}

func TestCheckVersionAcceptsUnreadableVersionAsOutdated(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_STDOUT", "git version custom-build\n")
	v, err := (Runner{Git: fakegit}).CheckVersion(t.Context())
	if err != nil {
		t.Fatalf("CheckVersion() err = %v, want nil", err)
	}
	if v != (Version{}) || !v.Outdated() {
		t.Errorf("CheckVersion() = %v, outdated %v; want zero Version, outdated", v, v.Outdated())
	}
}

func TestCheckVersionRejectsMissingGit(t *testing.T) {
	_, err := (Runner{Git: "/does/not/exist"}).CheckVersion(t.Context())
	coretest.AssertCode(t, err, core.GitMissing)
}

func TestOutdatedComparesReleaseCyclesOnly(t *testing.T) {
	cycle := latest
	cases := []struct {
		v    Version
		want bool
	}{
		{Version{Major: cycle.Major, Minor: cycle.Minor, Patch: 0}, false},
		{Version{Major: cycle.Major, Minor: cycle.Minor, Patch: 9}, false},
		{Version{Major: cycle.Major, Minor: cycle.Minor + 1, Patch: 0}, false},
		{Version{Major: cycle.Major + 1, Minor: 0, Patch: 0}, false},
		{Version{Major: cycle.Major, Minor: cycle.Minor - 1, Patch: 99}, true},
		{Version{Major: cycle.Major - 1, Minor: 99, Patch: 0}, true},
	}
	for _, c := range cases {
		if got := c.v.Outdated(); got != c.want {
			t.Errorf("%v.Outdated() = %v, want %v (latest %s)", c.v, got, c.want, Latest)
		}
	}
}

// clearVersionCache empties the per-process version cache for a test that
// changes what the fake git reports.
func clearVersionCache(t *testing.T) {
	t.Helper()
	versions.Clear()
	t.Cleanup(versions.Clear)
}

func TestCheckVersionRunsGitOncePerProcess(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_STDOUT", "git version 2.50.0\n")
	calls := 0
	r := Runner{Git: fakegit, Log: func(Entry) { calls++ }}
	for range 2 {
		if _, err := r.CheckVersion(t.Context()); err != nil {
			t.Fatalf("CheckVersion() err = %v, want nil", err)
		}
	}
	if calls != 1 {
		t.Errorf("git --version ran %d times, want 1", calls)
	}
}

func TestCheckVersionDoesNotCacheAFailure(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_EXIT", "1")
	calls := 0
	r := Runner{Git: fakegit, Log: func(Entry) { calls++ }}
	coretest.AssertCode(t, mustErr(r.CheckVersion(t.Context())), core.GitMissing)

	t.Setenv("FAKEGIT_EXIT", "0")
	t.Setenv("FAKEGIT_STDOUT", "git version 2.50.0\n")
	if _, err := r.CheckVersion(t.Context()); err != nil {
		t.Fatalf("CheckVersion() after an install err = %v, want nil", err)
	}
	if calls != 2 {
		t.Errorf("git --version ran %d times, want 2", calls)
	}
}

func mustErr(_ Version, err error) error { return err }
