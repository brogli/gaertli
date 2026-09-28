package remote

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
)

// exitingRunner is a wslRunner whose every wsl.exe start exits 1, as
// wsl.exe does for a distro it cannot start.
func exitingRunner(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "sh", "-c", "exit 1"), nil
}

// ubuntuOnly is a distro listing with Ubuntu alone.
func ubuntuOnly(context.Context) ([]Distro, error) {
	return []Distro{{Name: "Ubuntu"}}, nil
}

func TestFR_REPO_07_ADistroThatIsNotInstalledIsDistroNotFound(t *testing.T) {
	dc := NewDistroCore("Gone", "1.0.0", exitingRunner, &fakeConsent{accept: true}, nil, slog.Default())
	dc.listDistros = ubuntuOnly

	_, _, err := dc.Ensure(t.Context())

	ce := coretest.AsError(t, err, core.DistroNotFound)
	if ce.Key != "distroNotFound" || ce.Args["distro"] != "Gone" {
		t.Errorf("key, args = %q, %v; want distroNotFound, distro=Gone", ce.Key, ce.Args)
	}
}

// A WSL 1 distro is refused before anything starts in it: no deploy, no
// core.
func TestQA_PORT_02_ADistroOnWSL1IsRefusedBeforeAnyStart(t *testing.T) {
	started := func(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
		t.Errorf("wsl.exe started with %v, want no start in a WSL 1 distro", args)
		return exitingRunner(ctx, distro, args...)
	}
	dc := NewDistroCore("old", "1.0.0", started, &fakeConsent{accept: true}, nil, slog.Default())
	dc.listDistros = func(context.Context) ([]Distro, error) {
		return []Distro{{Name: "Old", Version: 1}}, nil
	}

	_, _, err := dc.Ensure(t.Context())

	ce := coretest.AsError(t, err, core.WslUnsupported)
	if ce.Key != "wsl1Unsupported" || ce.Args["distro"] != "old" {
		t.Errorf("key, args = %q, %v; want wsl1Unsupported, distro=old", ce.Key, ce.Args)
	}
}

// A distro whose row wsl.exe prints without a readable version is still
// installed: its failed start keeps its own error. One start lists the
// distros once, for the WSL 1 check and the failure alike.
func TestFR_REPO_07_AFailureInADistroWithAnUnreadableRowKeepsItsError(t *testing.T) {
	dc := NewDistroCore("Odd", "1.0.0", exitingRunner, &fakeConsent{accept: true}, nil, slog.Default())
	listings := 0
	dc.listDistros = func(context.Context) ([]Distro, error) {
		listings++
		return distrosOf([]byte("Ubuntu\r\nOdd\r\n"), []byte("  NAME    STATE      VERSION\r\n"+
			"* Ubuntu  Running    2\r\n"+
			"  Odd     Running    ?\r\n")), nil
	}

	_, _, err := dc.Ensure(t.Context())

	if _, exited := errors.AsType[*exec.ExitError](err); !exited {
		t.Errorf("err = %v, want wsl.exe's own exit error", err)
	}
	if listings != 1 {
		t.Errorf("distros listed %d times, want 1", listings)
	}
}

// wsl.exe matches distro names regardless of case, so a listed name in
// another case is installed.
func TestFR_REPO_07_AFailureInAnInstalledDistroKeepsItsError(t *testing.T) {
	dc := NewDistroCore("ubuntu", "1.0.0", exitingRunner, &fakeConsent{accept: true}, nil, slog.Default())
	dc.listDistros = ubuntuOnly

	_, _, err := dc.Ensure(t.Context())

	if _, exited := errors.AsType[*exec.ExitError](err); !exited {
		t.Errorf("err = %v, want wsl.exe's own exit error", err)
	}
}
