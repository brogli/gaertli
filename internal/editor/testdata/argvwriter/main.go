// Command argvwriter is a test fixture only (excluded from the module's own
// build): a stand-in editor that writes the rest of its argv to the file its
// first argument names and exits 1, as explorer.exe does after a successful
// open. While <file>.hold exists (at most 10s) it keeps running; it writes
// <file>.exited just before it exits.
package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	outFile := os.Args[1]
	_ = os.WriteFile(outFile, []byte(strings.Join(os.Args[2:], "\n")), 0o644)
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(outFile + ".hold"); err != nil {
			break
		}
	}
	_ = os.WriteFile(outFile+".exited", nil, 0o644)
	os.Exit(1)
}
