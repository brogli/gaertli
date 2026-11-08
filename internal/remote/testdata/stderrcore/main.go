// Command stderrcore is a test fixture: a normal core that also writes to
// stderr before serving, one line with a URL's credentials, so
// DistroCore's stderr logging and masking can be exercised.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/remote"
	"github.com/brogli/gaertli/internal/services"
)

func main() {
	fmt.Fprintln(os.Stderr, "starting up")
	fmt.Fprintln(os.Stderr, "remote: https://user:secret@host/repo")
	_ = remote.Serve(context.Background(), os.Stdin, os.Stdout, func(sink core.EventSink) core.API {
		return services.New(sink)
	})
}
