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

// wslRunner runs a command inside a distro: production wires it to the real
// wsl.exe (Windows-only, distrocore_windows.go/distrocore_other.go), tests
// wire it to a fake.
type wslRunner func(ctx context.Context, distro string, args ...string) (*exec.Cmd, error)

// Consent asks before the shell places its helper program and the nebegit
// command inside distro. Implemented by internal/app, since this package
// may not depend on Wails.
type Consent interface {
	Ask(distro string) bool
}

// deployTarget is where the core lands, scoped by app version, so a
// version bump lands at a fresh path needing its own consent and copy.
// $HOME is expanded by the distro's own sh, not here; shown to the user as
// "~/...".
func deployTarget(version string) string {
	return "$HOME/.cache/nebegit/" + version + "/nebegit-core"
}

// ensureDeployed makes sure d's distro has a working, current core binary
// at deployTarget, asking consent before any placement. A size match with
// the embedded core means it is already deployed — no consent asked.
func (d *DistroCore) ensureDeployed(ctx context.Context) error {
	size, err := d.deployedSize(ctx)
	if err != nil {
		return err
	}
	if size == int64(len(embeddedCore)) {
		return nil
	}
	return d.deployCore(ctx)
}

// deployCore asks consent and copies embeddedCore to deployTarget
// unconditionally, so ensure's version-mismatch retry replaces a stale
// binary whose size happened to match.
func (d *DistroCore) deployCore(ctx context.Context) error {
	if !d.consent.Ask(d.distro) {
		return core.ErrorfArgs(core.DeployRefused, "deployNeeded", map[string]string{"distro": d.distro}, "repos in %s need nebegit's helper; open the folder again to install it", d.distro)
	}
	return d.copyCore(ctx)
}

// deployedSize is deployTarget's size in d's distro, via `stat -c%s`; a
// nonexistent path is not an error, just -1.
func (d *DistroCore) deployedSize(ctx context.Context) (int64, error) {
	script := fmt.Sprintf("stat -c%%s %s 2>/dev/null || echo -1", deployTarget(d.version))
	cmd, err := d.run(ctx, d.distro, "sh", "-c", script)
	if err != nil {
		return 0, err
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// copyCore pipes embeddedCore into an executable deployTarget inside d's
// distro, through the distro's own sh rather than the \\wsl.localhost UNC
// path, which is slow to open.
func (d *DistroCore) copyCore(ctx context.Context) error {
	target := deployTarget(d.version)
	script := fmt.Sprintf("mkdir -p $(dirname %s) && cat > %s && chmod +x %s", target, target, target)
	cmd, err := d.run(ctx, d.distro, "sh", "-c", script)
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(embeddedCore)
	return cmd.Run()
}
