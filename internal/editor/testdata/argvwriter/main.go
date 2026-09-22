// Command argvwriter is a test fixture only (excluded from the module's own
// build): a stand-in editor that writes its own received argv to a file
// next to itself and exits.
package main

import (
	"os"
	"strings"
)

func main() {
	_ = os.WriteFile(os.Args[0]+".out", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
}
