package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brogli/gaertli/internal/core"
)

// stateFiles are what a stopped operation leaves in its worktree's git dir.
// The rebase directories, not REBASE_HEAD, mark a rebase stop: REBASE_HEAD is
// absent at a break or exec stop and lingers after a finished rebase.
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
// dir is gitDir, "" for none. File checks only, no subprocess. A merge or
// pick a rebase stopped on is the rebase; one the user started inside a
// stopped rebase is itself. A cherry-pick or revert sequence leaves only its
// sequencer directory once the user commits a stopped step. A stopped git am
// shares rebase-apply and reads as a rebase: git refuses rebase's verbs
// during an am, and nothing starts on top of it.
func operation(gitDir string) core.OperationKind {
	has := func(name string) bool { return fileExists(filepath.Join(gitDir, name)) }
	switch {
	case has("MERGE_HEAD") && !rebaseStoppedOnMerge(gitDir):
		return core.MergeOperation
	case has("CHERRY_PICK_HEAD") && (!has("rebase-merge") || stateSHA(gitDir, "CHERRY_PICK_HEAD") != stateSHA(gitDir, "REBASE_HEAD")):
		return core.CherryPickOperation
	case has("REVERT_HEAD"):
		return core.RevertOperation
	case has("sequencer"):
		return sequenceKind(gitDir)
	case has("rebase-merge") || has("rebase-apply"):
		return core.RebaseOperation
	}
	return ""
}

// rebaseStoppedOnMerge reports whether the last step an interactive rebase
// in gitDir ran is a merge.
func rebaseStoppedOnMerge(gitDir string) bool {
	data, _ := os.ReadFile(filepath.Join(gitDir, "rebase-merge", "done"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	command, _, _ := strings.Cut(lines[len(lines)-1], " ")
	return command == "merge" || command == "m"
}

// sequenceKind is the kind of the cherry-pick or revert sequence stopped in
// gitDir, told by its next step.
func sequenceKind(gitDir string) core.OperationKind {
	todo, _ := os.ReadFile(filepath.Join(gitDir, "sequencer", "todo"))
	if strings.HasPrefix(string(todo), "revert") {
		return core.RevertOperation
	}
	return core.CherryPickOperation
}

// stateFile is the file kind's stop leaves in a git dir, rebase-merge for a
// rebase.
func stateFile(kind core.OperationKind) string {
	for _, f := range stateFiles {
		if f.kind == kind {
			return f.name
		}
	}
	return ""
}

// rebaseStateFiles are the trimmed contents of the file name in each rebase
// directory gitDir holds, rebase-merge first.
func rebaseStateFiles(gitDir, name string) []string {
	var contents []string
	for _, f := range stateFiles {
		if f.kind != core.RebaseOperation {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(gitDir, f.name, name)); err == nil {
			contents = append(contents, strings.TrimSpace(string(data)))
		}
	}
	return contents
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
