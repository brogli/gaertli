package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", "../..", "repo root, relative to the package being generated")
	flag.Parse()

	files, err := Generate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gencore:", err)
		os.Exit(1)
	}
	for name, rel := range outputs {
		if err := os.WriteFile(filepath.Join(*root, rel), files[name], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gencore:", err)
			os.Exit(1)
		}
	}
}
