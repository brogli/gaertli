//go:build windows

package remote

import (
	"context"
	"os/exec"

	"github.com/brogli/nebegit/internal/proc"
)

// WslRun is the production wslRunner: wsl.exe -d <distro> -- args..., its
// console hidden (Research used) — the same call shape
// tools/wslspike/dial_windows.go already proved.
func WslRun(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "wsl.exe", append([]string{"-d", distro, "--"}, args...)...)
	cmd.SysProcAttr = proc.HiddenConsole()
	return cmd, nil
}
