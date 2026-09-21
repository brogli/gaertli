// Package fixture exercises every route and parameter shape gencore supports.
package fixture

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
)

type Q struct {
	Limit int `json:"limit"`
}

type Page struct {
	Rows []string `json:"rows"`
}

type API interface {
	Local(ctx context.Context) (Page, error)
	ByWorktree(ctx context.Context, wt core.WorktreeID, q Q) (Page, error)
	ByPath(ctx context.Context, path core.RepoPath) error
}
