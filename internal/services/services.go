// Package services implements core.API in-process: it validates input, runs
// git, and returns DTOs.
//
// One file group per bounded context: <context>.go holds the context's core
// methods, <context>_<topic>.go its own helpers; a file without a context
// stem holds helpers several contexts share.
package services

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/brogli/gaertli/internal/buildinfo"
	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// Core is the in-process core.
type Core struct {
	sink     core.EventSink
	sessions *session.Manager
	discover git.Runner // no Log, no session: ListWorktrees' and MainWorktree's stateless reads

	// editorProfile is the profile pushed by SetEditorProfile, set from a
	// different goroutine than the ones reading it.
	editorProfile atomic.Value
}

// New builds a core that reports events to sink.
func New(sink core.EventSink) *Core {
	return &Core{sink: sink, sessions: session.NewManager(sink)}
}

// Version implements core.API.
func (c *Core) Version(ctx context.Context) (core.Version, error) {
	v := core.Version{App: buildinfo.App, Commit: buildinfo.Commit, Protocol: core.Protocol}
	// A missing git is not this call's failure: opening a repo reports it.
	if gitVersion, err := c.discover.CheckVersion(ctx); err == nil {
		if gitVersion != (git.Version{}) {
			v.Git = gitVersion.String()
		}
		v.GitOutdated = gitVersion.Outdated()
	}
	return v, nil
}

// runLocked runs git with args in id's worktree under its write lock.
func (c *Core) runLocked(ctx context.Context, id core.WorktreeID, args ...string) (git.Result, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return git.Result{}, err
	}
	wt.Lock()
	defer wt.Unlock()
	return wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var _ core.API = (*Core)(nil)
