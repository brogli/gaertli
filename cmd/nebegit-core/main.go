// Command nebegit-core is the headless core (ADR-3): started by the shell
// inside a WSL distro, it serves core.API over JSON-RPC on stdio. Pure Go
// (TR-07). Logs go to stderr; stdout is the wire.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/brogli/nebegit/internal/buildinfo"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/proc"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	switch {
	case len(args) == 1 && args[0] == "--version":
		fmt.Println(buildinfo.App)
		return 0
	case len(args) == 1 && args[0] == "--stdio":
		return serve()
	case len(args) == 0:
		return launch(".")
	case len(args) == 1 && !strings.HasPrefix(args[0], "-"):
		return launch(args[0])
	default:
		return usage()
	}
}

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: nebegit-core [path] | --stdio | --version")
	return 2
}

// launch is nebegit-core's "given a path instead of --stdio" mode
// (ADR-3): resolves path to an absolute one, reads $WSL_DISTRO_NAME and the
// gui-path marker InstallLauncher wrote next to this binary's own real
// (symlink-resolved) location, builds the UNC path (remote.BuildWSLPath),
// and hands off to the single Windows instance exactly like a native
// `nebegit.exe <path>` launch would (spec 04's existing single-instance
// mechanism, unchanged).
func launch(path string) int {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fail(err)
	}
	distro := os.Getenv("WSL_DISTRO_NAME")
	if distro == "" {
		return fail(fmt.Errorf("not running inside a WSL distro shell (WSL_DISTRO_NAME is unset)"))
	}
	self, err := os.Executable()
	if err == nil {
		self, err = filepath.EvalSymlinks(self)
	}
	if err != nil {
		return fail(err)
	}
	guiExe, err := os.ReadFile(filepath.Join(filepath.Dir(self), "gui-path"))
	if err != nil {
		return fail(fmt.Errorf("run \"Install nebegit command in this distro\" from the app first: %w", err))
	}
	cmd := exec.Command(strings.TrimSpace(string(guiExe)), remote.BuildWSLPath(distro, abs))
	proc.Detach(cmd) // TR-12
	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	_ = cmd.Process.Release()
	return 0
}

// fail prints err to stderr, prefixed like every other nebegit-core
// user-facing failure, and returns launch's own "something went wrong"
// exit code.
func fail(err error) int {
	fmt.Fprintln(os.Stderr, "nebegit:", err)
	return 1
}

func serve() int {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	err := remote.Serve(context.Background(), os.Stdin, os.Stdout, func(sink core.EventSink) core.API {
		return services.New(sink)
	})
	if err != nil {
		slog.Error("core stopped", "err", err)
		return 1
	}
	return 0
}
