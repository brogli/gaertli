package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
)

// CommandLog implements core.API.
func (c *Core) CommandLog(ctx context.Context, id core.WorktreeID) ([]core.CommandLogEntry, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	return wt.CommandLog(), nil
}
