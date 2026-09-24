// Command fakegit stands in for the system git in tests. Its first argument
// selects a mode; environment variables control the rest:
//
//   - "env": dumps its environment, one "KEY=VALUE" per line.
//   - "argv": echoes the remaining args, one per line.
//   - "log" with FAKEGIT_STREAM_FILE set: copies that file's
//     0x00-terminated records to stdout one at a time, sleeping
//     FAKEGIT_STREAM_DELAY_MS between each, so a test can point Runner.Git
//     at fakegit for a whole session while only the log call streams.
//   - FAKEGIT_STDERR_FILE set (any argv): copies that file's \r/\n-terminated
//     records to stderr one at a time, sleeping FAKEGIT_STDERR_DELAY_MS
//     between each — the stderr counterpart of FAKEGIT_STREAM_FILE, for
//     Runner.StreamErr.
//   - otherwise: while sleeping FAKEGIT_SLEEP_MS (if set), touches
//     FAKEGIT_HEARTBEAT (if set) every 50ms; then writes FAKEGIT_STDOUT to
//     stdout and FAKEGIT_STDERR to stderr, then exits FAKEGIT_EXIT.
package main

import (
	"bytes"
	"fmt"
	"io"
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
		run(len(args) > 0 && args[0] == "log")
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

func run(isLog bool) {
	if hb := os.Getenv("FAKEGIT_HEARTBEAT"); hb != "" {
		go heartbeat(hb)
	}
	if ms, err := strconv.Atoi(os.Getenv("FAKEGIT_SLEEP_MS")); err == nil {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	if file := os.Getenv("FAKEGIT_STREAM_FILE"); isLog && file != "" {
		streamFile(file, "\x00", "FAKEGIT_STREAM_DELAY_MS", os.Stdout)
	} else {
		fmt.Print(os.Getenv("FAKEGIT_STDOUT"))
	}
	if file := os.Getenv("FAKEGIT_STDERR_FILE"); file != "" {
		streamFile(file, "\r\n", "FAKEGIT_STDERR_DELAY_MS", os.Stderr)
	} else {
		fmt.Fprint(os.Stderr, os.Getenv("FAKEGIT_STDERR"))
	}
	code, _ := strconv.Atoi(os.Getenv("FAKEGIT_EXIT"))
	os.Exit(code)
}

// streamFile copies path's records — each ending in one of delims — to w
// one at a time, sleeping the duration named by delayEnv between each. The
// shared implementation of FAKEGIT_STREAM_FILE (stdout, NUL-delimited) and
// FAKEGIT_STDERR_FILE (stderr, \r/\n-delimited).
func streamFile(path, delims, delayEnv string, w io.Writer) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	delayMS, _ := strconv.Atoi(os.Getenv(delayEnv))
	delay := time.Duration(delayMS) * time.Millisecond
	for len(data) > 0 {
		end := len(data) // an unterminated remainder is the last record
		if i := bytes.IndexAny(data, delims); i >= 0 {
			end = i + 1
		}
		w.Write(data[:end])
		data = data[end:]
		if delay > 0 {
			time.Sleep(delay)
		}
	}
}

// heartbeat touches path every 50ms until the process dies (killed on
// cancel, or exits normally alongside run).
func heartbeat(path string) {
	for {
		_ = os.WriteFile(path, nil, 0o644)
		time.Sleep(50 * time.Millisecond)
	}
}
