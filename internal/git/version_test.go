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
