// Command wslspike is the ADR-3 M0 spike harness (spec 06): produces
// pass/fail evidence for each of ADR-3's seven M0 spike items. Throwaway;
// nothing in the app imports it (Scope).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "serve":
		return runServe()
	case "dial":
		return runDialCmd(args[1:])
	case "args":
		printArgs(args[1:], os.Stdout)
		return 0
	default:
		return usage()
	}
}

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: wslspike serve | dial --local <path> | dial --distro <name> --remote <path> | args <arg>...")
	return 2
}

func runServe() int {
	if err := serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "wslspike serve:", err)
		return 1
	}
	return 0
}

func runDialCmd(args []string) int {
	fs := flag.NewFlagSet("dial", flag.ContinueOnError)
	local := fs.String("local", "", "path to a wslspike binary, run locally as \"serve\"")
	distro := fs.String("distro", "", "WSL distro name (Windows only)")
	remote := fs.String("remote", "", "server binary's path inside the distro")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg := dialConfig{local: *local, distro: *distro, remote: *remote}
	if _, err := runDial(context.Background(), cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "wslspike dial:", err)
		return 1
	}
	return 0
}
