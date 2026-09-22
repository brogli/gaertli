package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/jobs"
)

// Jobs implements core.API.
func (c *Core) Jobs(ctx context.Context, id core.WorktreeID) ([]core.JobInfo, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	return toJobInfos(wt.Repo.Jobs().List()), nil
}

// CancelJob implements core.API.
func (c *Core) CancelJob(ctx context.Context, id core.WorktreeID, jobID string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Repo.Jobs().Cancel(jobs.ID(jobID))
	return nil // Cancel's own false (unknown/finished) is not an error, Design
}

func toJobInfos(list []jobs.Job) []core.JobInfo {
	infos := make([]core.JobInfo, len(list))
	for i, j := range list {
		infos[i] = jobs.Info(j)
	}
	return infos
}
