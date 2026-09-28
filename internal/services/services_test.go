package services

import (
	"testing"

	"github.com/brogli/nebegit/internal/buildinfo"
)

func TestDIST_03_VersionReturnsAppAndCommit(t *testing.T) {
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
