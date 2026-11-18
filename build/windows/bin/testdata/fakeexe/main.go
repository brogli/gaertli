// Command fakeexe stands in for gaertli.exe in the script tests: it waits
// while a "hold" file exists in its working directory, so a test can check the
// script does not wait for it, then writes its arguments to argv.txt there. The rename makes the file
// appear whole: a test polling for it never reads it half written.
package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	for fileExists("hold") {
		time.Sleep(10 * time.Millisecond)
	}
	_ = os.WriteFile("argv.tmp", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	_ = os.Rename("argv.tmp", "argv.txt")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
