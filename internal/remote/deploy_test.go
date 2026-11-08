// Requirements: QA-PORT-02

package remote

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
)

// localRunner is a fake wslRunner: it ignores distro and runs args as a
// local command, so ensureDeployed's "sh -c ..." scripts run for real
// against a test-sandboxed $HOME.
func localRunner(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

// newDeployer is Ubuntu-22.04's DistroCore for version 1.0.0, deploying
// through localRunner.
func newDeployer(consent Consent) *DistroCore {
	return NewDistroCore("Ubuntu-22.04", "1.0.0", localRunner, consent, nil, slog.Default())
}

type fakeConsent struct {
	accept bool
	calls  []string // distro asked, once per Ask call
}

func (f *fakeConsent) Ask(distro string) bool {
	f.calls = append(f.calls, distro)
	return f.accept
}

// deployedPath expands deployTarget's literal "$HOME" the same way the
// sandboxed distro's own sh does, so the test can inspect the file it wrote.
func deployedPath(t *testing.T, version string) string {
	t.Helper()
	return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(deployTarget(version), "$HOME"))
}

func TestEnsureDeployedAsksOnceThenSkipsOnceDeployed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: true}
	dc := newDeployer(consent)

	if err := dc.ensureDeployed(t.Context()); err != nil {
		t.Fatal(err)
	}

	if len(consent.calls) != 1 || consent.calls[0] != "Ubuntu-22.04" {
		t.Fatalf("consent.calls = %+v, want one call for %q", consent.calls, "Ubuntu-22.04")
	}
	got, err := os.ReadFile(deployedPath(t, "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, embeddedCore) {
		t.Errorf("deployed content = %q, want %q", got, embeddedCore)
	}

	if err := dc.ensureDeployed(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(consent.calls) != 1 {
		t.Errorf("consent asked %d times on the second call, want 0 more (still 1 total)", len(consent.calls))
	}
}

func TestEnsureDeployedDeclinedConsentCopiesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: false}

	err := newDeployer(consent).ensureDeployed(t.Context())

	ce := coretest.AsError(t, err, core.DeployRefused)
	if ce.Args["distro"] != "Ubuntu-22.04" {
		t.Errorf("Args[distro] = %q, want %q", ce.Args["distro"], "Ubuntu-22.04")
	}
	if ce.Key != "deployNeeded" {
		t.Errorf("Key = %q, want deployNeeded", ce.Key)
	}
	if _, statErr := os.Stat(deployedPath(t, "1.0.0")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("target exists after a declined consent")
	}
}

func TestEnsureDeployedRecopiesOnSizeMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	target := deployedPath(t, "1.0.0")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("stale binary, wrong size"), 0o644); err != nil {
		t.Fatal(err)
	}
	consent := &fakeConsent{accept: true}

	if err := newDeployer(consent).ensureDeployed(t.Context()); err != nil {
		t.Fatal(err)
	}

	if len(consent.calls) != 1 {
		t.Fatalf("consent.calls = %d, want 1 (a size mismatch must ask again)", len(consent.calls))
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, embeddedCore) {
		t.Errorf("deployed content = %q, want %q (fresh copy)", got, embeddedCore)
	}
}
