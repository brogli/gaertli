//go:build windows

package remote

import (
	"context"
	"os"
	"os/exec"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/proc"
)

// ListDistros names the installed distros from wsl.exe --list --quiet and
// reads their WSL versions from --list --verbose (distrosOf). The versions
// are best-effort: if the verbose listing fails, every version is 0.
func ListDistros(ctx context.Context) ([]Distro, error) {
	if !wslPresent() {
		return nil, core.Errorf(core.WslNotFound, "WSL is not installed (wsl.exe not found on PATH)")
	}
	quiet, err := wslList(ctx, "--quiet")
	if err != nil {
		return nil, core.Errorf(core.WslNotFound, "wsl.exe --list --quiet failed: %v", err)
	}
	verbose, _ := wslList(ctx, "--verbose")
	return distrosOf(quiet, verbose), nil
}

// wslList runs wsl.exe --list with flag. WSL_UTF8=1 (WSL >= 0.64) makes the
// output UTF-8; decodeWSLText remains the fallback for older builds.
func wslList(ctx context.Context, flag string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", flag)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	cmd.SysProcAttr = proc.HiddenConsole()
	return cmd.Output()
}
