package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestFR_REPO_06_InstallLauncherWritesGuiPathAndSymlinksNebegit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: true}
	dc := NewDistroCore("Ubuntu-22.04", "1.0.0", localRunner, consent, nil)

	if err := dc.InstallLauncher(context.Background(), `C:\nebegit.exe`); err != nil {
		t.Fatal(err)
	}

	if len(consent.calls) != 1 {
		t.Fatalf("consent asked %d times, want 1", len(consent.calls))
	}

	target := deployedPath(t, "1.0.0")
	got, err := os.ReadFile(filepath.Join(filepath.Dir(target), "gui-path"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "/mnt/c/nebegit.exe" {
		t.Errorf("gui-path content = %q, want %q", got, "/mnt/c/nebegit.exe")
	}

	link := filepath.Join(os.Getenv("HOME"), ".local/bin/nebegit")
	linkTarget, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if linkTarget != target {
		t.Errorf("~/.local/bin/nebegit -> %q, want %q", linkTarget, target)
	}
}

func TestFR_REPO_06_InstallLauncherDeclinedConsentWritesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: false}
	dc := NewDistroCore("Ubuntu-22.04", "1.0.0", localRunner, consent, nil)

	err := dc.InstallLauncher(context.Background(), `C:\nebegit.exe`)

	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.DeployRefused {
		t.Fatalf("err = %v, want *core.Error{Code: DeployRefused}", err)
	}

	target := deployedPath(t, "1.0.0")
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(target), "gui-path")); !os.IsNotExist(statErr) {
		t.Error("gui-path file exists after a declined consent")
	}
	if _, statErr := os.Stat(filepath.Join(os.Getenv("HOME"), ".local/bin/nebegit")); !os.IsNotExist(statErr) {
		t.Error("~/.local/bin/nebegit exists after a declined consent")
	}
}
