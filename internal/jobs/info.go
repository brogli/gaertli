package jobs

import (
	"errors"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// Info converts j to its JSON DTO.
func Info(j Job) core.JobInfo {
	info := core.JobInfo{
		ID:        string(j.ID),
		Kind:      j.Kind,
		State:     string(j.State),
		Progress:  j.Percent,
		Message:   j.Message,
		StartedAt: j.StartedAt.Format(time.RFC3339Nano),
	}
	if !j.EndedAt.IsZero() {
		info.EndedAt = j.EndedAt.Format(time.RFC3339Nano)
	}
	if j.Err != nil {
		var ce *core.Error
		if errors.As(j.Err, &ce) {
			info.Error = ce
		} else {
			info.Error = core.Errorf(core.Internal, "%v", j.Err)
		}
	}
	return info
}
