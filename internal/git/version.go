package git

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/brogli/nebegit/internal/core"
)

// Version is git's own version, as reported by "git --version".
type Version struct{ Major, Minor, Patch int }

// MinVersion is the oldest supported git.
var MinVersion = Version{Major: 2, Minor: 42, Patch: 0}

// Less reports whether v is older than o.
func (v Version) Less(o Version) bool {
	return cmp.Or(cmp.Compare(v.Major, o.Major), cmp.Compare(v.Minor, o.Minor), cmp.Compare(v.Patch, o.Patch)) < 0
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// versions holds the version of every git executable that passed
// CheckVersion, by Runner.Git: git does not change while the app runs, and
// every worktree open asks.
var versions sync.Map

// CheckVersion runs "git --version" and returns *core.Error{Code: GitTooOld}
// if git cannot be started, its output cannot be parsed, or it is older
// than MinVersion. A passed check is remembered for the process; a failed
// one is repeated, so an installed or upgraded git is seen on the next
// call.
func (r Runner) CheckVersion(ctx context.Context) (Version, error) {
	if v, ok := versions.Load(r.Git); ok {
		return v.(Version), nil
	}
	v, err := r.checkVersion(ctx)
	if err != nil {
		return Version{}, err
	}
	versions.Store(r.Git, v)
	return v, nil
}

func (r Runner) checkVersion(ctx context.Context) (Version, error) {
	result, err := r.Run(ctx, Call{Args: []string{"--version"}, ReadOnly: true})
	if err != nil {
		return Version{}, core.Errorf(core.GitTooOld, "git --version: %v", err)
	}
	v, err := parseVersion(string(result.Stdout))
	if err != nil {
		return Version{}, core.Errorf(core.GitTooOld, "%v", err)
	}
	if v.Less(MinVersion) {
		args := map[string]string{"version": v.String(), "minVersion": MinVersion.String()}
		return Version{}, core.ErrorfArgs(core.GitTooOld, "gitTooOld", args, "git %s is older than the minimum %s", v, MinVersion)
	}
	return v, nil
}

// parseVersion extracts Major.Minor.Patch from the first three
// dot-separated numeric groups of "git --version"'s stdout, e.g.
// "git version 2.43.0" or "git version 2.43.0.windows.1".
func parseVersion(out string) (Version, error) {
	for field := range strings.FieldsSeq(out) {
		nums, ok := leadingDotted3(field)
		if ok {
			return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
		}
	}
	return Version{}, fmt.Errorf("git: unrecognized version output %q", out)
}

// leadingDotted3 parses the first three dot-separated numeric groups of s.
func leadingDotted3(s string) ([3]int, bool) {
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i := range nums {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return [3]int{}, false
		}
		nums[i] = n
	}
	return nums, true
}
