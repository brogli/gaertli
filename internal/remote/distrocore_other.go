//go:build !windows

package remote

import (
	"context"
	"os/exec"

	"github.com/brogli/nebegit/internal/core"
)

// WslRun only exists on Windows (WSL is Windows-only).
func WslRun(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	return nil, core.Errorf(core.WslNotFound, "WSL is Windows-only")
}
