// Command fakeexe stands in for nebegit.exe in the script tests: it sleeps
// a second, so a test can check the script does not wait for it, then writes
// its arguments to argv.txt in its working directory.
package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	time.Sleep(time.Second)
	_ = os.WriteFile("argv.txt", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
}
