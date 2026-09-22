//go:build windows

package remote

import (
	"context"
	"os"
	"os/exec"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/proc"
)

// ListDistros runs wsl.exe --list --verbose and parses it (FR-REPO-07's
// picker). WSL_UTF8=1 (Microsoft-documented, WSL >= 0.64) makes the output
// UTF-8 on a current build; decodeWSLText remains the fallback for older
// ones. CREATE_NO_WINDOW (TR-12) hides its console.
func ListDistros(ctx context.Context) ([]Distro, error) {
	if !wslPresent() {
		return nil, core.Errorf(core.WslNotFound, "WSL is not installed (wsl.exe not found on PATH)")
	}

	cmd := exec.CommandContext(ctx, "wsl.exe", "-l", "-v")
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	cmd.SysProcAttr = proc.HiddenConsole()
	out, err := cmd.Output()
	if err != nil {
		return nil, core.Errorf(core.WslNotFound, "wsl.exe --list --verbose failed: %v", err)
	}
	return parseDistroList(out)
}
