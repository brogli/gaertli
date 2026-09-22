package remote

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
)

// wslRunner is the one seam every wsl.exe-spawning call goes through:
// production wires it to the real wsl.exe (Windows-only,
// distrocore_windows.go/distrocore_other.go), tests wire it to a fake.
type wslRunner func(ctx context.Context, distro string, args ...string) (*exec.Cmd, error)

// Consent asks before the shell places its helper program inside a distro,
// naming exactly where. Implemented by internal/app, since this package may
// not depend on Wails.
type Consent interface {
	Ask(distro, targetPath string) bool
}

// deployTarget is where the core lands, scoped by app version, so a
// version bump lands at a fresh path needing its own consent and copy.
// $HOME is expanded by the distro's own sh, not here; shown to the user as
// "~/...".
func deployTarget(version string) string {
	return "$HOME/.cache/nebegit/" + version + "/nebegit-core"
}

// ensureDeployed makes sure distro has a working, current core binary at
// deployTarget(version), asking consent before any placement. A size match
// with the embedded core means it is already deployed — no consent asked.
func ensureDeployed(ctx context.Context, run wslRunner, consent Consent, distro, version string) error {
	target := deployTarget(version)
	size, err := remoteSize(ctx, run, distro, target)
	if err != nil {
		return err
	}
	if size == int64(len(embeddedCore)) {
		return nil
	}
	return deployCore(ctx, run, consent, distro, target)
}

// deployCore asks consent and copies embeddedCore to target
// unconditionally, so DistroCore's version-mismatch retry replaces a stale
// binary whose size happened to match.
func deployCore(ctx context.Context, run wslRunner, consent Consent, distro, target string) error {
	if !consent.Ask(distro, target) {
		return core.Errorf(core.DeployRefused, "copying nebegit's helper into %s was declined", distro)
	}
	return copyCore(ctx, run, distro, target, embeddedCore)
}

// remoteSize is target's size in distro, via `stat -c%s`; a nonexistent
// path is not an error, just -1.
func remoteSize(ctx context.Context, run wslRunner, distro, target string) (int64, error) {
	script := fmt.Sprintf("stat -c%%s %s 2>/dev/null || echo -1", target)
	cmd, err := run(ctx, distro, "sh", "-c", script)
	if err != nil {
		return 0, err
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// copyCore pipes payload into target inside distro: wsl.exe -d <distro> --
// sh -c 'mkdir -p $(dirname <target>) && cat > <target> && chmod +x
// <target>', stdin = payload — never writing through the \\wsl.localhost
// UNC path, which is slow to open.
func copyCore(ctx context.Context, run wslRunner, distro, target string, payload []byte) error {
	script := fmt.Sprintf("mkdir -p $(dirname %s) && cat > %s && chmod +x %s", target, target, target)
	cmd, err := run(ctx, distro, "sh", "-c", script)
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(payload)
	return cmd.Run()
}
