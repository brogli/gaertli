package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	out := flag.String("out", "../../frontend/src/locales/errorKeys.generated.json", "output path, relative to the package being generated")
	flag.Parse()

	data, err := Generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "errorkeysgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "errorkeysgen:", err)
		os.Exit(1)
	}
}
