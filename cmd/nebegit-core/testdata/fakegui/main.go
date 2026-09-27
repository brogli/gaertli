// Command fakegui is a test fixture: a stand-in for the Windows app
// nebegit-core's launch mode hands off to. It sleeps briefly, so a test can
// check nebegit-core does not wait for it, then writes its arguments to
// argv.txt in its working directory.
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
