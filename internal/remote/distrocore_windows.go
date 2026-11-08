//go:build windows

package remote

import (
	"context"
	"os/exec"

	"github.com/brogli/gaertli/internal/proc"
)

// WslRun is the production wslRunner: wsl.exe -d <distro> -- args..., with
// its console hidden.
func WslRun(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "wsl.exe", append([]string{"-d", distro, "--"}, args...)...)
	cmd.SysProcAttr = proc.HiddenConsole()
	return cmd, nil
}
