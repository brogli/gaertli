//go:build !windows

package remote

import (
	"context"

	"github.com/brogli/gaertli/internal/core"
)

// ListDistros only exists on Windows (WSL is Windows-only).
func ListDistros(ctx context.Context) ([]Distro, error) {
	return nil, core.Errorf(core.WslNotFound, "WSL is Windows-only")
}
