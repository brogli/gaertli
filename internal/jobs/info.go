package jobs

import (
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// Info converts j to its JSON DTO.
func Info(j Job) core.JobInfo {
	info := core.JobInfo{
		ID:        string(j.ID),
		Kind:      j.Kind,
		Branch:    j.Branch,
		State:     string(j.State),
		Progress:  j.Percent,
		Message:   j.Message,
		StartedAt: j.StartedAt.Format(time.RFC3339Nano),
		Result:    j.Result,
	}
	if !j.EndedAt.IsZero() {
		info.EndedAt = j.EndedAt.Format(time.RFC3339Nano)
	}
	if j.Err != nil {
		info.Error = core.ErrorOf(j.Err)
	}
	return info
}
