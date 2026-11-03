// Command argvwriter is a test fixture only (excluded from the module's own
// build): a stand-in editor that writes the rest of its argv to the file its
// first argument names and exits 1, as explorer.exe does after a successful
// open.
package main

import (
	"os"
	"strings"
)

func main() {
	_ = os.WriteFile(os.Args[1], []byte(strings.Join(os.Args[2:], "\n")), 0o644)
	os.Exit(1)
}
