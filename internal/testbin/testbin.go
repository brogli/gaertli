// Package testbin builds a Go helper binary for a subprocess test, from a
// TestMain, the same way for every package that needs one.
package testbin

import (
	"fmt"
	"os"
	"os/exec"
)

// Build compiles pkg (relative to the calling test's package directory)
// to out, pure Go (TR-07).
func Build(out, pkg string) error {
	build := exec.Command("go", "build", "-o", out, pkg)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, output)
	}
	return nil
}
