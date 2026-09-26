// Package testbin builds Go helper binaries for subprocess tests, the same
// way for every package that needs one.
package testbin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Bin is one helper binary: Pkg is built pure Go (CGO_ENABLED=0) and its
// path stored in *Path.
type Bin struct {
	Path *string
	Pkg  string // package path, relative to the calling test's directory
}

// Main builds bins into a temp directory, runs m and removes the directory.
// It is a whole TestMain, or part of one:
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
		*bin.Path = filepath.Join(dir, Exe(binaryName(bin.Pkg)))
		if err := build(*bin.Path, bin.Pkg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	m.Run()
	_ = os.RemoveAll(dir)
}

// Exe names a binary so exec finds it: on Windows it tries a name only with
// a PATHEXT extension added.
func Exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// binaryName is the last element of pkg's directory: the test's own
// directory name when pkg is ".".
func binaryName(pkg string) string {
	dir, err := filepath.Abs(pkg)
	if err != nil {
		return "testbin"
	}
	return filepath.Base(dir)
}

func build(out, pkg string) error {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, output)
	}
	return nil
}
