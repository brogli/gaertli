// Command gaertli is the desktop GUI: the Wails shell with the core in-process.
package main

import (
	"os"

	"github.com/brogli/gaertli/frontend"
	"github.com/brogli/gaertli/internal/app"
)

func main() {
	if err := app.Run(frontend.Dist, os.Args); err != nil {
		os.Exit(1)
	}
}
