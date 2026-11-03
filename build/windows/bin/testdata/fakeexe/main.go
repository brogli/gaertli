// Command fakeexe stands in for nebegit.exe in the script tests: it sleeps
// a second, so a test can check the script does not wait for it, then writes
// its arguments to argv.txt in its working directory. The rename makes the file
// appear whole: a test polling for it never reads it half written.
package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	time.Sleep(time.Second)
	_ = os.WriteFile("argv.tmp", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	_ = os.Rename("argv.tmp", "argv.txt")
}
