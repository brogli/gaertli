package main

import (
	"fmt"
	"io"
)

// printArgs prints one argv element per line (case 4: the owner compares
// this against what they passed, to catch silent WSL-interop path
// translation).
func printArgs(args []string, w io.Writer) {
	for _, a := range args {
		_, _ = fmt.Fprintln(w, a)
	}
}
