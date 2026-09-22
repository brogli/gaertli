package remote

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

func TestADR_3_DeployTarget(t *testing.T) {
	got := deployTarget("1.4.0")
	want := "$HOME/.cache/nebegit/1.4.0/nebegit-core"
	if got != want {
		t.Errorf("deployTarget(%q) = %q, want %q", "1.4.0", got, want)
	}
}

// localRunner is the fake wslRunner cases 6-8 use: it ignores distro and
// runs args as a real local command, so ensureDeployed's own "sh -c ..."
// scripts run for real against a test-sandboxed $HOME.
func localRunner(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

type consentCall struct{ distro, target string }

type fakeConsent struct {
	accept bool
	calls  []consentCall
}

func (f *fakeConsent) Ask(distro, target string) bool {
	f.calls = append(f.calls, consentCall{distro, target})
	return f.accept
}

// deployedPath expands deployTarget's literal "$HOME" the same way the
// sandboxed distro's own sh does, so the test can inspect the file it wrote.
func deployedPath(t *testing.T, version string) string {
	t.Helper()
	return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(deployTarget(version), "$HOME"))
}

func TestQA_PORT_02_EnsureDeployedAsksOnceThenSkipsOnceDeployed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: true}

	if err := ensureDeployed(context.Background(), localRunner, consent, "Ubuntu-22.04", "1.0.0"); err != nil {
		t.Fatal(err)
	}

	wantTarget := "$HOME/.cache/nebegit/1.0.0/nebegit-core"
	if len(consent.calls) != 1 || consent.calls[0] != (consentCall{"Ubuntu-22.04", wantTarget}) {
		t.Fatalf("consent.calls = %+v, want one call for (Ubuntu-22.04, %s)", consent.calls, wantTarget)
	}
	got, err := os.ReadFile(deployedPath(t, "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, embeddedCore) {
		t.Errorf("deployed content = %q, want %q", got, embeddedCore)
	}

	if err := ensureDeployed(context.Background(), localRunner, consent, "Ubuntu-22.04", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if len(consent.calls) != 1 {
		t.Errorf("consent asked %d times on the second call, want 0 more (still 1 total)", len(consent.calls))
	}
}

func TestQA_PORT_02_EnsureDeployedDeclinedConsentCopiesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	consent := &fakeConsent{accept: false}

	err := ensureDeployed(context.Background(), localRunner, consent, "Ubuntu-22.04", "1.0.0")

	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.DeployRefused {
		t.Fatalf("err = %v, want *core.Error{Code: DeployRefused}", err)
	}
	if _, statErr := os.Stat(deployedPath(t, "1.0.0")); !os.IsNotExist(statErr) {
		t.Errorf("target exists after a declined consent")
	}
}

func TestADR_3_EnsureDeployedRecopiesOnSizeMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	target := deployedPath(t, "1.0.0")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("stale binary, wrong size"), 0o644); err != nil {
		t.Fatal(err)
	}
	consent := &fakeConsent{accept: true}

	if err := ensureDeployed(context.Background(), localRunner, consent, "Ubuntu-22.04", "1.0.0"); err != nil {
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
