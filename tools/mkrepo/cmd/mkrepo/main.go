// Command mkrepo builds (or reuses) the reference repo tools/bench measures
// against.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/brogli/gaertli/tools/mkrepo"
)

func main() {
	out := flag.String("out", ".bench/reference-repo", "reference repo path")
	force := flag.Bool("force", false, "rebuild even if the fingerprint matches")
	flag.Parse()

	if *force {
		_ = os.RemoveAll(*out) // Build's own cache check would no-op otherwise
	}
	if err := mkrepo.Build(context.Background(), *out, mkrepo.Params); err != nil {
		fmt.Fprintln(os.Stderr, "mkrepo:", err)
		os.Exit(1)
	}
}
