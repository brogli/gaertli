package git

import (
	"context"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
)

func TestTR_04_CheckVersionAcceptsInstalledGit(t *testing.T) {
	v, err := (Runner{}).CheckVersion(context.Background())
	if err != nil {
		t.Fatalf("CheckVersion() err = %v, want nil", err)
	}
	if v.Less(MinVersion) {
		t.Errorf("CheckVersion() = %v, want >= %v", v, MinVersion)
	}
}

func TestTR_04_CheckVersionRejectsTooOldGit(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "git version 2.30.0\n")
	_, err := (Runner{Git: fakegit}).CheckVersion(context.Background())
	coretest.AssertCode(t, err, core.GitTooOld)
}

func TestTR_04_CheckVersionRejectsMissingGit(t *testing.T) {
	_, err := (Runner{Git: "/does/not/exist"}).CheckVersion(context.Background())
	coretest.AssertCode(t, err, core.GitTooOld)
}
