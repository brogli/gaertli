package git

import (
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
)

func TestTR_04_CheckVersionAcceptsInstalledGit(t *testing.T) {
	v, err := (Runner{}).CheckVersion(t.Context())
	if err != nil {
		t.Fatalf("CheckVersion() err = %v, want nil", err)
	}
	if v.Less(MinVersion) {
		t.Errorf("CheckVersion() = %v, want >= %v", v, MinVersion)
	}
}

func TestTR_04_CheckVersionRejectsTooOldGit(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	_, err := (Runner{Git: fakegit}).CheckVersion(t.Context())
	ce := coretest.AsError(t, err, core.GitTooOld)
	if ce.Args["version"] != "2.30.0" || ce.Args["minVersion"] != MinVersion.String() {
		t.Errorf("Args = %+v, want version=2.30.0 minVersion=%s", ce.Args, MinVersion)
	}
	if ce.Key != "gitTooOld" {
		t.Errorf("Key = %q, want gitTooOld", ce.Key)
	}
}

func TestTR_04_CheckVersionRejectsMissingGit(t *testing.T) {
	_, err := (Runner{Git: "/does/not/exist"}).CheckVersion(t.Context())
	coretest.AssertCode(t, err, core.GitTooOld)
}

// clearVersionCache empties the per-process version cache for a test that
// changes what the fake git reports.
func clearVersionCache(t *testing.T) {
	t.Helper()
	versions.Clear()
	t.Cleanup(versions.Clear)
}

func TestTR_04_CheckVersionRunsGitOncePerProcess(t *testing.T) {
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

func TestTR_04_CheckVersionDoesNotCacheAFailure(t *testing.T) {
	clearVersionCache(t)
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	calls := 0
	r := Runner{Git: fakegit, Log: func(Entry) { calls++ }}
	coretest.AssertCode(t, mustErr(r.CheckVersion(t.Context())), core.GitTooOld)

	t.Setenv("FAKEGIT_STDOUT", "git version 2.50.0\n")
	if _, err := r.CheckVersion(t.Context()); err != nil {
		t.Fatalf("CheckVersion() after an upgrade err = %v, want nil", err)
	}
	if calls != 2 {
		t.Errorf("git --version ran %d times, want 2", calls)
	}
}

func mustErr(_ Version, err error) error { return err }
