package services

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

const detailsFormat = "%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%cn%x1f%ce%x1f%ct%x1f%B%x00"

// CommitDetails implements core.API.
func (c *Core) CommitDetails(ctx context.Context, id core.WorktreeID, sha string) (core.CommitDetails, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.CommitDetails{}, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{
			"show", "--format=" + detailsFormat, "--name-status", "-z", "-M",
			"--first-parent", "--end-of-options", sha,
		},
	})
	if err != nil {
		return core.CommitDetails{}, err
	}
	return parseCommitDetails(result.Stdout)
}

// parseCommitDetails parses detailsFormat's header (9 fields, %x1f-
// separated, %x00-terminated) followed by --name-status -z -M records. Two
// bytes separate them: -z turns the usual header/diff blank line into a
// second 0x00, then git appends its own '\n', as it does after every
// custom --format.
func parseCommitDetails(stdout []byte) (core.CommitDetails, error) {
	before, after, ok := bytes.Cut(stdout, []byte{0})
	if !ok {
		return core.CommitDetails{}, core.Errorf(core.Internal, "commit details: no header terminator")
	}
	header := strings.Split(string(before), "\x1f")
	if len(header) != 9 {
		return core.CommitDetails{}, core.Errorf(core.Internal, "commit details header has %d fields, want 9", len(header))
	}
	authorAt, err := strconv.ParseInt(header[4], 10, 64)
	if err != nil {
		return core.CommitDetails{}, core.Errorf(core.Internal, "commit details authorAt: %v", err)
	}
	committerAt, err := strconv.ParseInt(header[7], 10, 64)
	if err != nil {
		return core.CommitDetails{}, core.Errorf(core.Internal, "commit details committerAt: %v", err)
	}

	files, err := parseNameStatus(bytes.TrimPrefix(after, []byte{0, '\n'}))
	if err != nil {
		return core.CommitDetails{}, err
	}

	return core.CommitDetails{
		SHA: header[0], Parents: strings.Fields(header[1]),
		Author: header[2], AuthorEmail: header[3], AuthorAt: authorAt,
		Committer: header[5], CommitterEmail: header[6], CommitterAt: committerAt,
		Body: strings.TrimSuffix(header[8], "\n"), Files: files,
	}, nil
}

// parseNameStatus parses --name-status -z -M records: "<status>\0<path>\0"
// or, for a rename/copy, "<status><similarity>\0<old>\0<new>\0".
func parseNameStatus(data []byte) ([]core.ChangedFile, error) {
	tokens := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	var files []core.ChangedFile
	for i := 0; i < len(tokens); {
		status := string(tokens[i])
		if status == "" {
			i++
			continue
		}
		if status[0] == 'R' || status[0] == 'C' {
			if i+2 >= len(tokens) {
				return nil, core.Errorf(core.Internal, "name-status: truncated rename record")
			}
			files = append(files, core.ChangedFile{Status: status[:1], OldPath: string(tokens[i+1]), Path: string(tokens[i+2])})
			i += 3
			continue
		}
		if i+1 >= len(tokens) {
			return nil, core.Errorf(core.Internal, "name-status: truncated record")
		}
		files = append(files, core.ChangedFile{Status: status, Path: string(tokens[i+1])})
		i += 2
	}
	return files, nil
}
