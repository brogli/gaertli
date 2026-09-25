package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// repoRoot is the repo root, relative to internal/core, where go:generate
// runs this.
const repoRoot = "../.."

func main() {
	files, err := Generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gencore:", err)
		os.Exit(1)
	}
	for _, out := range outputs {
		if err := os.WriteFile(filepath.Join(repoRoot, out.path), files[out.path], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gencore:", err)
			os.Exit(1)
		}
	}
}
