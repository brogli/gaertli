// Package testbin builds Go helper binaries for subprocess tests, the same
// way for every package that needs one.
package testbin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Bin is one helper binary: Pkg is built pure Go (TR-07) and its path
// stored in *Path.
type Bin struct {
	Path *string
	Pkg  string // package path, relative to the calling test's directory
}

// Main builds bins into a temp directory, runs m, and exits with its code.
// It is a whole TestMain:
//
//	func TestMain(m *testing.M) {
//		testbin.Main(m, testbin.Bin{Path: &fakegit, Pkg: "./testdata/fakegit"})
//	}
func Main(m *testing.M, bins ...Bin) {
	dir, err := os.MkdirTemp("", "testbin")
	if err != nil {
		panic(err)
	}
	for _, bin := range bins {
		*bin.Path = filepath.Join(dir, binaryName(bin.Pkg))
		if err := build(*bin.Path, bin.Pkg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// binaryName is pkg's last path element, or the test's own directory name
// when pkg is "." or "..".
func binaryName(pkg string) string {
	name := filepath.Base(pkg)
	if name != "." && name != ".." {
		return name
	}
	wd, err := os.Getwd()
	if err != nil {
		return "testbin"
	}
	return filepath.Base(filepath.Join(wd, pkg))
}

func build(out, pkg string) error {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, output)
	}
	return nil
}
