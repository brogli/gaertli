// Command gaertli-core is the headless core: started by the shell inside a
// WSL distro, it serves core.API over JSON-RPC on stdio. Pure Go. Logs go
// to stderr; stdout is the wire.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/brogli/gaertli/internal/buildinfo"
	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/remote"
	"github.com/brogli/gaertli/internal/services"
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
	default:
		return usage()
	}
}

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: gaertli-core --stdio | --version")
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
