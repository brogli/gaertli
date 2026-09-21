// Command nebegit-core is the headless core (ADR-3): started by the shell
// inside a WSL distro, it serves core.API over JSON-RPC on stdio. Pure Go
// (TR-07). Logs go to stderr; stdout is the wire.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/brogli/nebegit/internal/buildinfo"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) != 1 {
		return usage()
	}
	switch args[0] {
	case "--version":
		fmt.Println(buildinfo.App)
		return 0
	case "--stdio":
		return serve()
	default:
		return usage()
	}
}

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: nebegit-core --stdio | --version")
	return 2
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
