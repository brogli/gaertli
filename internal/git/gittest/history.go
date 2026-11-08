package gittest

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// Step is one commit of History.
type Step struct {
	Branch  string // branch to commit on; "master" if empty
	From    string // branch a new Branch forks from; empty for a root commit
	Merge   string // branch whose tip becomes the second parent
	Message string
	Author  string            // author name; the test identity if empty. The email is the lowercased name at gaertli.invalid.
	Files   map[string]string // path to content, set in this commit
	Delete  []string          // paths removed in this commit
}

// History writes steps into dir, a repo without commits, in one git
// fast-import run, then checks out master. It returns each commit's SHA in
// order. A merge commit starts from its first parent's tree: Files must
// bring in what the second parent added.
func History(t *testing.T, dir string, steps ...Step) []core.SHA {
	t.Helper()
	marks := filepath.Join(t.TempDir(), "marks")
	_, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir:   dir,
		Args:  []string{"fast-import", "--quiet", "--export-marks=" + marks},
		Stdin: fastImportStream(steps, time.Now().Unix()-int64(len(steps))),
	})
	if err != nil {
		t.Fatalf("git fast-import: %v", err)
	}
	Run(t, dir, "reset", "--hard", "--quiet")
	return shasFromMarks(t, marks, len(steps))
}

func fastImportStream(steps []Step, start int64) []byte {
	const identity = identityName + " <" + identityEmail + ">"
	var s bytes.Buffer
	for i, c := range steps {
		when := start + int64(i)
		author := identity
		if c.Author != "" {
			author = fmt.Sprintf("%s <%s@gaertli.invalid>", c.Author, strings.ToLower(c.Author))
		}
		msg := strings.TrimRight(c.Message, "\n") + "\n"
		fmt.Fprintf(&s, "commit refs/heads/%s\nmark :%d\nauthor %s %d +0000\ncommitter %s %d +0000\ndata %d\n%s",
			cmp.Or(c.Branch, "master"), i+1, author, when, identity, when, len(msg), msg)
		if c.From != "" {
			fmt.Fprintf(&s, "from refs/heads/%s\n", c.From)
		}
		if c.Merge != "" {
			fmt.Fprintf(&s, "merge refs/heads/%s\n", c.Merge)
		}
		for path, content := range c.Files {
			fmt.Fprintf(&s, "M 100644 inline %s\ndata %d\n%s\n", path, len(content), content)
		}
		for _, path := range c.Delete {
			fmt.Fprintf(&s, "D %s\n", path)
		}
		s.WriteString("\n")
	}
	return s.Bytes()
}

// shasFromMarks reads fast-import's exported marks (":N sha" lines).
func shasFromMarks(t *testing.T, path string, n int) []core.SHA {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	shas := make([]core.SHA, n)
	for line := range strings.Lines(string(data)) {
		var mark int
		var sha string
		if _, err := fmt.Sscanf(line, ":%d %s", &mark, &sha); err != nil {
			t.Fatalf("marks line %q: %v", line, err)
		}
		shas[mark-1] = core.SHA(sha)
	}
	return shas
}

// Copy duplicates the repo in src, working tree and .git, into a new temp
// dir. The copy holds no reference to src unless src has linked worktrees.
func Copy(t *testing.T, src string) *Repo {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	// The copy has new mtimes and inodes; without a refresh, plumbing such as
	// diff-files reports every tracked file modified.
	Run(t, dir, "update-index", "-q", "--unmerged", "--refresh")
	return &Repo{Dir: dir}
}
