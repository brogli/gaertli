package app

import (
	"context"

	"github.com/brogli/gaertli/internal/git"
)

// gitRuns reports whether runner's git can be run, whatever its version.
func gitRuns(ctx context.Context, runner git.Runner) bool {
	_, err := runner.CheckVersion(ctx)
	return err == nil
}
