package services

import (
	"path/filepath"

	"github.com/brogli/gaertli/internal/core"
)

// stateFiles are what a stopped operation leaves in its worktree's git dir,
// in the order operation checks them. MERGE_HEAD, CHERRY_PICK_HEAD and
// REVERT_HEAD are documented pseudorefs (gitrevisions(7)). rebase-merge/
// and rebase-apply/ are documented in no man page, yet they are the only
// state every rebase stop leaves (REBASE_HEAD is absent at a break or exec
// stop and lingers after a finished rebase), and git's own status command
// reads them.
var stateFiles = []struct {
	name string
	kind core.OperationKind
}{
	{"MERGE_HEAD", core.MergeOperation},
	{"CHERRY_PICK_HEAD", core.CherryPickOperation},
	{"REVERT_HEAD", core.RevertOperation},
	{"rebase-merge", core.RebaseOperation},
	{"rebase-apply", core.RebaseOperation},
}

// operation names the operation in progress in the worktree whose own git
// dir is gitDir, "" for none. File checks only, no subprocess. A head file
// wins over the rebase directory: a cherry-pick started inside a stopped
// rebase is continued as a cherry-pick; the rebase shows once it is done.
func operation(gitDir string) core.OperationKind {
	for _, f := range stateFiles {
		if fileExists(filepath.Join(gitDir, f.name)) {
			return f.kind
		}
	}
	return ""
}

// integrateRefusal is the error the app gives before git runs a merge or
// rebase: an operation in progress, else a detached HEAD; nil when git may
// decide. An operation is reported before a detached HEAD (a rebase is both).
func integrateRefusal(op core.OperationKind, detached bool) *core.Error {
	switch {
	case op != "":
		return core.ErrorfArgs(core.GitFailed, "operationInProgress", map[string]string{"operation": string(op)}, "A %s is in progress. Continue or abort it first.", op)
	case detached:
		return core.ErrorfArgs(core.GitFailed, "detachedHead", nil, "HEAD is detached. Check out a branch first.")
	}
	return nil
}

// asStop reclassifies err, from a step that left an operation in progress,
// as a stop: Code Conflicted with a conflicted file, else Code Stopped,
// whatever err's code was (a timeout included).
func asStop(err error, conflicted bool) error {
	ce := core.ErrorOf(err)
	ce.Code = core.Stopped
	if conflicted {
		ce.Code = core.Conflicted
	}
	return ce
}

// remapIfPending reclassifies git's own failure (Code GitFailed) as
// Conflicted when an operation is pending: git stopped mid-way rather than
// refusing outright. Cherry-pick, revert and the pull's integrate step use it.
func remapIfPending(err error, pending bool) error {
	if ce := gitFailure(err); ce != nil && pending {
		ce.Code = core.Conflicted
	}
	return err
}

// skippable reports whether op takes --skip: a cherry-pick or revert. Merge
// has none; rebase's is not offered.
func skippable(op core.OperationKind) bool {
	return op == core.CherryPickOperation || op == core.RevertOperation
}
