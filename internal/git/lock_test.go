// Requirements: FR-UX-16

package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// Every kind of write against a held lock gets the same code and key, and
// succeeds once the lock is gone.
func TestAnotherProgramsLockIsDetected(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string) []string // returns the call's args
		lock  string                                  // path under .git
	}{
		{
			name: "commit blocked by index.lock",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				gittest.Commit(t, dir, "a.txt", "one", "init")
				writeFile(t, dir, "a.txt", "two")
				gittest.Run(t, dir, "add", "--", "a.txt")
				return []string{"commit", "-m", "second"}
			},
			lock: "index.lock",
		},
		{
			name: "stage blocked by index.lock",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				gittest.Commit(t, dir, "a.txt", "one", "init")
				writeFile(t, dir, "b.txt", "new")
				return []string{"add", "--", "b.txt"}
			},
			lock: "index.lock",
		},
		{
			name: "checkout blocked by HEAD.lock",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				gittest.Commit(t, dir, "a.txt", "one", "init")
				gittest.Run(t, dir, "branch", "other")
				return []string{"switch", "other"}
			},
			lock: "HEAD.lock",
		},
		{
			name: "branch create blocked by its own ref lock",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				gittest.Commit(t, dir, "a.txt", "one", "init")
				return []string{"branch", "--", "topic"}
			},
			lock: "refs/heads/topic.lock",
		},
		{
			name: "fetch into a ref blocked by that ref's lock",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				gittest.Commit(t, dir, "a.txt", "one", "init")
				remote := gittest.New(t)
				gittest.Commit(t, remote.Dir, "a.txt", "one", "init")
				return []string{"fetch", "--", remote.Dir, "HEAD:refs/heads/fetched"}
			},
			lock: "refs/heads/fetched.lock",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := gittest.New(t)
			args := c.setup(t, repo.Dir)
			lockPath := filepath.Join(repo.Dir, ".git", c.lock)
			if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: repo.Dir, Args: args})

			ce := coretest.AsError(t, err, core.Locked)
			if ce.Key != "locked" {
				t.Errorf("Key = %q, want %q", ce.Key, "locked")
			}
			if want := filepath.Base(c.lock); ce.Args["lock"] != want {
				t.Errorf("Args[lock] = %q, want %q", ce.Args["lock"], want)
			}

			if err := os.Remove(lockPath); err != nil {
				t.Fatal(err)
			}
			if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: repo.Dir, Args: args}); err != nil {
				t.Errorf("retry after the lock is gone: %v", err)
			}
		})
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
