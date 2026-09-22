// Command badcore is a test fixture only (excluded from the module's own
// build: go tooling ignores testdata/ directories): a core.API that reports
// a wrong Protocol on Version, everything else delegating to the real
// services.Core. Built by internal/remote's own tests (case 10) to exercise
// DistroCore.Ensure's version-mismatch handling without a real distro.
package main

import (
	"context"
	"os"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
)

func main() {
	_ = remote.Serve(context.Background(), os.Stdin, os.Stdout, func(sink core.EventSink) core.API {
		return badVersion{services.New(sink)}
	})
}

type badVersion struct{ *services.Core }

func (badVersion) Version(context.Context) (core.Version, error) {
	return core.Version{App: "badcore", Protocol: core.Protocol + 1}, nil
}
