// Command fakegui is a test fixture only (excluded from the module's own
// build): a stand-in for the Windows GUI binary nebegit-core's launch mode
// hands off to (spec 31 case 7). It sleeps briefly, so the test can prove
// nebegit-core does not wait for it, then writes its own argv to argv.txt
// in its working directory and exits.
package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	time.Sleep(300 * time.Millisecond)
	_ = os.WriteFile("argv.txt", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
}
