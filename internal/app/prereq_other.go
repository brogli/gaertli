//go:build !windows

package app

import (
	"fmt"
	"os"
)

// showFatal shows msg before any window exists. Linux can only reach this
// for the git-missing case (Research used: WebKitGTK links before Go code
// runs); macOS is untested/best effort (QA-PORT-04).
func showFatal(title, msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, msg)
}
