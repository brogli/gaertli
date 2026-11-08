package jobs

import "github.com/brogli/gaertli/internal/core"

// Info converts j to its JSON DTO. An UpdateBranchJob's
// core.UpdateBranchResult is the one result a job returns.
func Info(j Job) core.JobInfo {
	info := core.JobInfo{
		ID:       string(j.ID),
		Kind:     j.Kind,
		Branch:   j.Branch,
		State:    j.State,
		Progress: j.Percent,
		Message:  j.Message,
	}
	if result, ok := j.Result.(core.UpdateBranchResult); ok {
		info.Result = &result
	}
	if j.Err != nil {
		info.Error = core.ErrorOf(j.Err)
	}
	return info
}
