// Command fakegit stands in for the system git in internal/git's
// integration tests. Its argv selects a mode; otherwise it is
// controlled by environment variables (spec 02, task 2):
//
//   - argv[0] == "env": dumps its environment, one "KEY=VALUE" per line.
//   - argv[0] == "argv": echoes the remaining args, one per line.
//   - otherwise: while sleeping FAKEGIT_SLEEP_MS (if set), touches
//     FAKEGIT_HEARTBEAT (if set) every 50ms; then writes FAKEGIT_STDOUT to
//     stdout and FAKEGIT_STDERR to stderr, then exits FAKEGIT_EXIT.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	args := os.Args[1:]
	switch {
	case len(args) > 0 && args[0] == "env":
		dumpEnv()
	case len(args) > 0 && args[0] == "argv":
		echoArgv(args[1:])
	default:
		run()
	}
}

func dumpEnv() {
	for _, kv := range os.Environ() {
		fmt.Println(kv)
	}
}

func echoArgv(args []string) {
	for _, a := range args {
		fmt.Println(a)
	}
}

func run() {
	if hb := os.Getenv("FAKEGIT_HEARTBEAT"); hb != "" {
		go heartbeat(hb)
	}
	if ms, err := strconv.Atoi(os.Getenv("FAKEGIT_SLEEP_MS")); err == nil {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	fmt.Print(os.Getenv("FAKEGIT_STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("FAKEGIT_STDERR"))
	code, _ := strconv.Atoi(os.Getenv("FAKEGIT_EXIT"))
	os.Exit(code)
}

// heartbeat touches path every 50ms until the process dies (killed on
// cancel, or exits normally alongside run).
func heartbeat(path string) {
	for {
		_ = os.WriteFile(path, nil, 0o644)
		time.Sleep(50 * time.Millisecond)
	}
}
