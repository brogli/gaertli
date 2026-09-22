// Command fakegit stands in for the system git in tests. Its argv selects a
// mode; environment variables control the rest:
//
//   - argv[0] == "env": dumps its environment, one "KEY=VALUE" per line.
//   - argv[0] == "argv": echoes the remaining args, one per line.
//   - argv[0] == "log" and FAKEGIT_STREAM_FILE is set: copies that file's
//     0x00-terminated records to stdout one at a time, sleeping
//     FAKEGIT_STREAM_DELAY_MS between each, so a test can point Runner.Git
//     at fakegit for a whole session while only the log call streams.
//   - otherwise: while sleeping FAKEGIT_SLEEP_MS (if set), touches
//     FAKEGIT_HEARTBEAT (if set) every 50ms; then writes FAKEGIT_STDOUT to
//     stdout and FAKEGIT_STDERR to stderr, then exits FAKEGIT_EXIT.
//     FAKEGIT_SLEEP_ONCE_MS applies only to the first call that finds
//     FAKEGIT_SLEEP_ONCE_FILE missing, so one call in a session can be slow
//     and the rest fast.
package main

import (
	"bytes"
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
	if ms, ok := sleepOnce(); ok {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	if file := os.Getenv("FAKEGIT_STREAM_FILE"); isLog && file != "" {
		streamRecords(file)
	} else {
		fmt.Print(os.Getenv("FAKEGIT_STDOUT"))
	}
	fmt.Fprint(os.Stderr, os.Getenv("FAKEGIT_STDERR"))
	code, _ := strconv.Atoi(os.Getenv("FAKEGIT_EXIT"))
	os.Exit(code)
}

// sleepOnce reports the ms to sleep, and whether to sleep at all: only for
// the first call that finds FAKEGIT_SLEEP_ONCE_FILE missing (it then
// creates it, so later calls in the same test see it and skip the sleep).
func sleepOnce() (ms int, ok bool) {
	file := os.Getenv("FAKEGIT_SLEEP_ONCE_FILE")
	n, err := strconv.Atoi(os.Getenv("FAKEGIT_SLEEP_ONCE_MS"))
	if file == "" || err != nil {
		return 0, false
	}
	if _, statErr := os.Stat(file); statErr == nil {
		return 0, false
	}
	if writeErr := os.WriteFile(file, nil, 0o644); writeErr != nil {
		return 0, false
	}
	return n, true
}

// streamRecords copies path's 0x00-terminated records to stdout one at a
// time, sleeping FAKEGIT_STREAM_DELAY_MS between each.
func streamRecords(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	delayMS, _ := strconv.Atoi(os.Getenv("FAKEGIT_STREAM_DELAY_MS"))
	delay := time.Duration(delayMS) * time.Millisecond
	for _, record := range splitRecords(data) {
		os.Stdout.Write(record)
		if delay > 0 {
			time.Sleep(delay)
		}
	}
}

// splitRecords splits data into pieces ending at (and including) each 0x00
// byte, with any unterminated remainder as a final piece.
func splitRecords(data []byte) [][]byte {
	var records [][]byte
	for {
		i := bytes.IndexByte(data, 0)
		if i < 0 {
			if len(data) > 0 {
				records = append(records, data)
			}
			return records
		}
		records = append(records, data[:i+1])
		data = data[i+1:]
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
