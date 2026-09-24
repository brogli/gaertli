//go:build !windows

package app

import (
	"fmt"
	"os"
)

// showFatal shows msg before any window exists. A missing web view never
// reaches it on Linux: WebKitGTK links before Go code runs. macOS is
// untested, best effort.
func showFatal(title, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, msg)
}
