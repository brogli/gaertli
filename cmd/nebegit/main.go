// Command nebegit is the desktop GUI: the Wails shell with the core in-process.
package main

import (
	"os"

	"github.com/brogli/nebegit/frontend"
	"github.com/brogli/nebegit/internal/app"
)

func main() {
	if err := app.Run(frontend.Dist, os.Args); err != nil {
		os.Exit(1)
	}
}
