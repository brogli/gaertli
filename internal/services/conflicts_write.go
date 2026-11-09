package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// SaveConflict implements core.API.
func (c *Core) SaveConflict(ctx context.Context, id core.WorktreeID, path, text, fingerprint string, markResolved bool) (string, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return "", err
	}
	wt.Lock() // writes the working file and the index
	defer wt.Unlock()

	full, err := worktreeFile(wt.Path, path)
	if err != nil {
		return "", err
	}
	current, err := os.ReadFile(full)
	if err != nil {
		return "", core.Errorf(core.Internal, "read %s: %v", path, err)
	}
	if fingerprintOf(current) != fingerprint {
		return "", core.ErrorfArgs(core.GitFailed, "fileChangedSinceOpened", map[string]string{"path": path}, "%s changed since it was opened.", path)
	}
	if markResolved {
		attrs, err := conflictAttrs(ctx, wt.Runner, wt.Path, path)
		if err != nil {
			return "", err
		}
		if hasMarkerLine(text, attrs.markerSize) {
			return "", core.ErrorfArgs(core.GitFailed, "conflictMarkersLeft", map[string]string{"path": path}, "%s still has conflict markers.", path)
		}
	}

	out := []byte(text)
	if usesCRLF(current) {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	if err := os.WriteFile(full, out, 0); err != nil { // the file exists: its mode stays
		return "", core.Errorf(core.Internal, "write %s: %v", path, err)
	}
	if markResolved {
		if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"add", "--", literalPath(path)}}); err != nil {
			return "", err
		}
	}
	return fingerprintOf(out), nil
}

// ResolveConflict implements core.API.
func (c *Core) ResolveConflict(ctx context.Context, id core.WorktreeID, path string, choice core.ConflictChoice) error {
	literal := literalPath(path)
	var steps [][]string
	switch choice {
	case core.ChooseCurrent:
		steps = [][]string{{"checkout", "--ours", "--", literal}, {"add", "--", literal}}
	case core.ChooseIncoming:
		steps = [][]string{{"checkout", "--theirs", "--", literal}, {"add", "--", literal}}
	case core.ChooseDelete:
		steps = [][]string{{"rm", "--quiet", "--", literal}}
	default:
		return core.Errorf(core.Internal, "unknown conflict choice %q", choice)
	}
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	// Checking out a side of a path without unmerged entries would
	// overwrite its working file.
	if _, err := unmergedStages(ctx, wt.Runner, wt.Path, path); err != nil {
		return err
	}
	for _, args := range steps {
		if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args}); err != nil {
			return err
		}
	}
	return nil
}

// ReopenConflict implements core.API.
func (c *Core) ReopenConflict(ctx context.Context, id core.WorktreeID, path string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()

	if operation(wt.GitDir) == "" {
		return core.ErrorfArgs(core.GitFailed, "noOperation", nil, "No merge, rebase, cherry-pick or revert is in progress.")
	}
	literal := literalPath(path)
	record, err := resolveUndo(ctx, wt.Runner, wt.Path, literal)
	if err != nil {
		return err
	}
	stages, ok := record[path]
	stillUnmerged, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Args: []string{"ls-files", "-u", "-z", "--", literal}})
	if err != nil {
		return err
	}
	if !ok || len(indexEntries(stillUnmerged.Stdout)) > 0 {
		return core.ErrorfArgs(core.GitFailed, "notReopenable", map[string]string{"path": path}, "%s was not resolved during this operation.", path)
	}
	if err := restoreStages(ctx, wt, stages); err != nil {
		return err
	}
	var flag string
	switch {
	case isRegularMode(stages[2].mode) && isRegularMode(stages[3].mode):
		flag = "-m" // writes the merge with its markers
	case stages[2].mode != "":
		flag = "--ours" // a binary or symlink conflict leaves the current side in the working tree
	case stages[3].mode != "":
		flag = "--theirs" // the current side deleted the file; git left the incoming version
	default:
		return nil
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"checkout", flag, "--", literal}})
	return err
}

// resolvedPaths lists the paths in the resolve-undo record that are no
// longer conflicted, sorted.
func resolvedPaths(ctx context.Context, wt *session.Worktree, conflicted []core.ChangedFile) ([]string, error) {
	record, err := resolveUndo(ctx, wt.Runner, wt.Path)
	if err != nil {
		return nil, err
	}
	for _, f := range conflicted {
		delete(record, f.Path)
	}
	return slices.Sorted(maps.Keys(record)), nil
}

// isRegularMode reports whether an index mode is a regular file's.
func isRegularMode(mode string) bool {
	return mode == "100644" || mode == "100755"
}

// restoreStages puts the stages of one path, as the resolve-undo record has them, back into the
// index. update-index --unresolve is no use: before git 2.43 it fails for a path removed from
// the index and takes "--" as a path.
func restoreStages(ctx context.Context, wt *session.Worktree, stages map[int]indexEntry) error {
	var entries []byte
	for i, stage := range slices.Sorted(maps.Keys(stages)) {
		e := stages[stage]
		if i == 0 { // mode 0 drops the path's resolved entry first
			entries = fmt.Appendf(entries, "0 %s 0\t%s\x00", strings.Repeat("0", len(e.oid)), e.path)
		}
		entries = fmt.Appendf(entries, "%s %s %d\t%s\x00", e.mode, e.oid, e.stage, e.path)
	}
	_, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"update-index", "-z", "--index-info"}, Stdin: entries})
	return err
}

// resolveUndo returns the stages each path in the resolve-undo record had before it was
// resolved. pathspecs limit the paths listed.
func resolveUndo(ctx context.Context, r git.Runner, dir string, pathspecs ...string) (map[string]map[int]indexEntry, error) {
	args := []string{"ls-files", "--resolve-undo", "-z"}
	if len(pathspecs) > 0 {
		args = append(append(args, "--"), pathspecs...)
	}
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true, Args: args})
	if err != nil {
		return nil, err
	}
	record := map[string]map[int]indexEntry{}
	for _, e := range indexEntries(result.Stdout) {
		if record[e.path] == nil {
			record[e.path] = map[int]indexEntry{}
		}
		record[e.path][e.stage] = e
	}
	return record, nil
}

// fingerprintOf is the hex sha256 of a working file's bytes.
func fingerprintOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// usesCRLF reports whether file has CRLF line breaks and no bare LF.
func usesCRLF(file []byte) bool {
	crlf := bytes.Count(file, []byte("\r\n"))
	return crlf > 0 && crlf == bytes.Count(file, []byte("\n"))
}

// hasMarkerLine reports whether text has a conflict-start or conflict-end marker line of size
// characters.
func hasMarkerLine(text string, size int) bool {
	runs := newMarkerRuns(size)
	return slices.ContainsFunc(splitLines(text), func(line string) bool {
		return isMarker(line, runs.open) || isMarker(line, runs.close)
	})
}
