//go:build !windows

package remote

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
)

// ListDistros only exists on Windows (WSL is Windows-only); same pattern as
// tools/wslspike/dial_other.go.
func ListDistros(ctx context.Context) ([]Distro, error) {
	return nil, core.Errorf(core.WslNotFound, "WSL is Windows-only")
}
