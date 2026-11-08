package app

import (
	"context"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// GitWarning is an outdated git a window warns about. Distro is "" for the
// host's git.
type GitWarning struct {
	Version string `json:"version"` // "" when git did not report it readably
	Latest  string `json:"latest"`
	Distro  string `json:"distro"`
}

// noteGit queues a warning if api's git is outdated, once per run for
// distro's git ("" for the host's).
func (r *Registry) noteGit(ctx context.Context, distro string, api core.API) {
	v, err := api.Version(ctx)
	if err != nil || !v.GitOutdated {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gitWarned[distro] {
		return
	}
	r.gitWarned[distro] = true
	r.gitWarnings = append(r.gitWarnings, GitWarning{Version: v.Git, Latest: git.LatestCycle(), Distro: distro})
}

// TakeGitWarnings returns the warnings not yet shown, oldest first, and
// drops them: only the first window to ask shows them.
func (r *Registry) TakeGitWarnings() []GitWarning {
	r.mu.Lock()
	defer r.mu.Unlock()
	warnings := r.gitWarnings
	r.gitWarnings = nil
	return warnings
}
