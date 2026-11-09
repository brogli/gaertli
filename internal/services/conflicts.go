package services

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/session"
)

// binarySniffLen is how much of a file git looks at for a NUL byte.
const binarySniffLen = 8000

// defaultMarkerSize is the length of a conflict marker line without a
// conflict-marker-size attribute.
const defaultMarkerSize = 7

// gitSubmodule and gitSymlink are the index modes of a submodule pointer
// and a symbolic link.
const (
	gitSubmodule = "160000"
	gitSymlink   = "120000"
)

// unmerged is one index entry of a conflicted path.
type unmerged struct{ mode, oid string }

// Conflict implements core.API.
func (c *Core) Conflict(ctx context.Context, id core.WorktreeID, path string) (core.ConflictFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.ConflictFile{}, err
	}
	stages, err := unmergedStages(ctx, wt.Runner, wt.Path, path)
	if err != nil {
		return core.ConflictFile{}, err
	}
	out := core.ConflictFile{
		Path: path, Kind: conflictKind(stages),
		Base: core.ConflictSide{Present: stages[1] != nil}, Current: core.ConflictSide{Present: stages[2] != nil},
		Incoming:   core.ConflictSide{Present: stages[3] != nil},
		ConflictID: conflictID(stages),
	}
	if err := labelSides(ctx, wt, &out); err != nil {
		return core.ConflictFile{}, err
	}
	if out.Kind != "" {
		return out, nil
	}
	return readConflictText(ctx, wt.Runner, wt.Path, out, stages)
}

// MergeDraft implements core.API.
func (c *Core) MergeDraft(ctx context.Context, id core.WorktreeID, path string) (*core.MergeDraft, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	draft := wt.MergeDraft(path)
	if draft == nil {
		return nil, nil
	}
	current, err := conflictIDOf(ctx, wt.Runner, wt.Path, path)
	if err != nil {
		return nil, err
	}
	if current == "" || draft.ConflictID != current {
		wt.SetMergeDraft(path, nil)
		return nil, nil
	}
	return draft, nil
}

// SetMergeDraft implements core.API.
func (c *Core) SetMergeDraft(ctx context.Context, id core.WorktreeID, path string, draft *core.MergeDraft) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if draft == nil {
		wt.SetMergeDraft(path, nil)
		return nil
	}
	current, err := conflictIDOf(ctx, wt.Runner, wt.Path, path)
	if err != nil {
		return err
	}
	if current != "" && draft.ConflictID == current {
		wt.SetMergeDraft(path, draft)
	}
	return nil
}

// conflictIDOf is the ConflictID of path's conflict, "" if path is not
// conflicted.
func conflictIDOf(ctx context.Context, r git.Runner, dir, path string) (string, error) {
	stages, err := unmergedStages(ctx, r, dir, path)
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Key == "notConflicted" {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return conflictID(stages), nil
}

// conflictID identifies a conflict by its index entries.
func conflictID(stages map[int]*unmerged) string {
	var id strings.Builder
	for stage := 1; stage <= 3; stage++ {
		if s := stages[stage]; s != nil {
			id.WriteString(s.mode + " " + s.oid)
		}
		id.WriteString("\n")
	}
	return id.String()
}

// unmergedStages returns path's index entries by stage (1 base, 2 current,
// 3 incoming); a missing stage is nil. No entry is GitFailed (key
// notConflicted).
func unmergedStages(ctx context.Context, r git.Runner, dir, path string) (map[int]*unmerged, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"ls-files", "-u", "-z", "--", literalPath(path)},
	})
	if err != nil {
		return nil, err
	}
	stages := map[int]*unmerged{}
	for _, e := range indexEntries(result.Stdout) {
		if e.path == path {
			stages[e.stage] = &unmerged{mode: e.mode, oid: e.oid}
		}
	}
	if len(stages) == 0 {
		return nil, core.ErrorfArgs(core.GitFailed, "notConflicted", map[string]string{"path": path}, "%s is not conflicted.", path)
	}
	return stages, nil
}

// indexEntry is one record of ls-files -z output.
type indexEntry struct {
	mode, oid string
	stage     int
	path      string
}

// indexEntries parses ls-files -z records of the form "<mode> <oid> <stage>\t<path>", skipping
// any that do not fit.
func indexEntries(stdout []byte) []indexEntry {
	var entries []indexEntry
	for record := range strings.SplitSeq(string(stdout), "\x00") {
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			continue
		}
		if stage, err := strconv.Atoi(fields[2]); err == nil {
			entries = append(entries, indexEntry{mode: fields[0], oid: fields[1], stage: stage, path: path})
		}
	}
	return entries
}

// conflictKind classifies a conflict by its index shape alone; "" means the
// contents decide.
func conflictKind(stages map[int]*unmerged) core.ConflictKind {
	hasMode := func(mode string) bool {
		for _, s := range stages {
			if s.mode == mode {
				return true
			}
		}
		return false
	}
	switch {
	case hasMode(gitSubmodule):
		return core.ConflictSubmodule
	case len(stages) == 1:
		return core.ConflictRename
	case stages[1] != nil && len(stages) == 2:
		return core.ConflictDeleted
	case hasMode(gitSymlink):
		return core.ConflictSymlink
	}
	return ""
}

// readConflictText classifies a two-sided conflict by its contents and, for
// text, fills the four texts.
func readConflictText(ctx context.Context, r git.Runner, dir string, out core.ConflictFile, stages map[int]*unmerged) (core.ConflictFile, error) {
	attrs, err := conflictAttrs(ctx, r, dir, out.Path)
	if err != nil {
		return core.ConflictFile{}, err
	}
	if attrs.binary {
		out.Kind = core.ConflictBinary
		return out, nil
	}
	full, err := worktreeFile(dir, out.Path)
	if err != nil {
		return core.ConflictFile{}, err
	}
	working, err := os.ReadFile(full)
	if err != nil {
		return core.ConflictFile{}, core.Errorf(core.Internal, "read %s: %v", out.Path, err)
	}
	texts := [][]byte{working}
	for stage := 1; stage <= 3; stage++ {
		var blob []byte
		if stages[stage] != nil {
			if blob, err = catFiltered(ctx, r, dir, out.Path, stages[stage].oid); err != nil {
				return core.ConflictFile{}, err
			}
		}
		texts = append(texts, blob)
	}

	if out.Kind = textKind(texts); out.Kind != core.ConflictText {
		return out, nil
	}
	text := func(b []byte) string { return string(b) }
	if usesCRLF(working) {
		// SaveConflict writes CRLF back only for such a file; any other
		// file's texts pass unchanged, so its line ends survive a save.
		text = func(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }
	}
	out.Result = text(texts[0])
	out.Base.Content, out.Current.Content, out.Incoming.Content = text(texts[1]), text(texts[2]), text(texts[3])
	out.MarkerSize = attrs.markerSize
	toCurrent, toIncoming, err := baseDiffs(ctx, r, dir, out)
	if err != nil {
		return core.ConflictFile{}, err
	}
	if out.Changes, err = conflictRegions(ctx, r, dir, out, toCurrent, toIncoming); err != nil {
		return core.ConflictFile{}, err
	}
	out.CurrentChanges = sideChanges(out.Base, out.Current.Content, toCurrent)
	out.IncomingChanges = sideChanges(out.Base, out.Incoming.Content, toIncoming)
	out.Fingerprint = fingerprintOf(working)
	return out, nil
}

// textKind is ConflictText unless one of texts holds a NUL (binary) or is
// not valid UTF-8.
func textKind(texts [][]byte) core.ConflictKind {
	for _, t := range texts {
		if bytes.IndexByte(t[:min(len(t), binarySniffLen)], 0) >= 0 {
			return core.ConflictBinary
		}
	}
	for _, t := range texts {
		if !utf8.Valid(t) {
			return core.ConflictNotUTF8
		}
	}
	return core.ConflictText
}

// catFiltered returns the blob oid in its working-tree form for path.
func catFiltered(ctx context.Context, r git.Runner, dir, path, oid string) ([]byte, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"cat-file", "--filters", "--path=" + path, oid},
	})
	return result.Stdout, err
}

// mergeAttrs are the attributes that steer how git merges a path.
type mergeAttrs struct {
	binary     bool // the merge attribute is unset or "binary"
	markerSize int  // length of a conflict marker line
}

// conflictAttrs reads path's merge and conflict-marker-size attributes.
func conflictAttrs(ctx context.Context, r git.Runner, dir, path string) (mergeAttrs, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"check-attr", "-z", "merge", "conflict-marker-size", "--", path},
	})
	if err != nil {
		return mergeAttrs{}, err
	}
	attrs := mergeAttrs{markerSize: defaultMarkerSize}
	fields := strings.Split(string(result.Stdout), "\x00") // path, attribute, value, ...
	for i := 0; i+2 < len(fields); i += 3 {
		name, value := fields[i+1], fields[i+2]
		switch name {
		case "merge":
			attrs.binary = value == "unset" || value == "binary"
		case "conflict-marker-size":
			if size, err := strconv.Atoi(value); err == nil && size > 0 {
				attrs.markerSize = size
			}
		}
	}
	return attrs, nil
}

// labelSides sets the operation and what the current and incoming sides come from. Current is
// HEAD, except in a rebase, where it is the commit rebased onto.
func labelSides(ctx context.Context, wt *session.Worktree, out *core.ConflictFile) error {
	op := operation(wt.GitDir)
	out.Operation = core.Operation(op)

	var err error
	if op != "rebase" {
		if out.Current.SHA, err = verifyRev(ctx, wt.Runner, wt.Path, "HEAD"); err != nil {
			return err
		}
		if out.Current.Ref, _, err = headBranch(ctx, wt.Runner, wt.Path); err != nil {
			return err
		}
	}

	switch op {
	case "merge":
		out.Incoming.SHA = stateSHA(wt.GitDir, "MERGE_HEAD")
		out.Incoming.Ref, err = refAt(ctx, wt.Runner, wt.Path, out.Incoming.SHA)
	case "rebase":
		out.Current.SHA = cmp.Or(
			stateSHA(wt.GitDir, filepath.Join("rebase-merge", "onto")),
			stateSHA(wt.GitDir, filepath.Join("rebase-apply", "onto")),
		)
		out.Current.Ref, err = refAt(ctx, wt.Runner, wt.Path, out.Current.SHA)
		out.Incoming.SHA = stateSHA(wt.GitDir, "REBASE_HEAD")
	case "cherry-pick":
		out.Incoming.SHA = stateSHA(wt.GitDir, "CHERRY_PICK_HEAD")
	case "revert":
		out.Incoming.SHA = stateSHA(wt.GitDir, "REVERT_HEAD")
	}
	if err != nil {
		return err
	}
	if op != "merge" && out.Incoming.SHA != "" {
		out.Incoming.Subject, err = subjectOf(ctx, wt.Runner, wt.Path, out.Incoming.SHA)
	}
	return err
}

// stateSHA returns the first line of the state file name in gitDir, "" if
// there is none.
func stateSHA(gitDir, name string) core.SHA {
	data, err := os.ReadFile(filepath.Join(gitDir, name))
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return core.SHA(strings.TrimSpace(line))
}

// refAt returns the short name of a ref pointing at sha: a local branch,
// else a remote branch, else a tag, the first by name. "" if there is none.
func refAt(ctx context.Context, r git.Runner, dir string, sha core.SHA) (string, error) {
	if sha == "" {
		return "", nil
	}
	// for-each-ref sorts by full name, and heads < remotes < tags.
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"for-each-ref", "--points-at=" + string(sha), "--format=%(refname)", "refs/heads", "refs/remotes", "refs/tags"},
	})
	if err != nil {
		return "", err
	}
	for name := range strings.FieldsSeq(string(result.Stdout)) {
		ref := core.RefName(name)
		if ref.IsRemoteBranch() && strings.HasSuffix(name, "/HEAD") {
			continue // a remote's default-branch alias
		}
		return ref.Short(), nil
	}
	return "", nil
}

// subjectOf returns commit sha's subject line.
func subjectOf(ctx context.Context, r git.Runner, dir string, sha core.SHA) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"log", "--no-walk", "--format=%s", "--end-of-options", string(sha)},
	})
	return strings.TrimSpace(string(result.Stdout)), err
}
