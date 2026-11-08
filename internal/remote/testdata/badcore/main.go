// Command badcore is a test fixture: a core.API that reports a wrong
// Protocol on Version, everything else delegating to services.Core. It
// exercises DistroCore.Ensure's version-mismatch handling without a distro.
package main

import (
	"context"
	"os"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/remote"
	"github.com/brogli/gaertli/internal/services"
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
