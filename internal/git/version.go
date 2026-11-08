package git

import (
	"cmp"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/brogli/gaertli/internal/core"
)

// Version is git's own version, as reported by "git --version".
type Version struct{ Major, Minor, Patch int }

// Latest is the newest git release (endoflife.date's newest cycle), the one
// the app supports. Renovate bumps it.
const Latest = "2.56.0" // renovate: datasource=endoflife-date depName=git

var latest = must(parseVersion(Latest))

// LatestCycle is Latest's release cycle, e.g. "2.56".
func LatestCycle() string { return fmt.Sprintf("%d.%d", latest.Major, latest.Minor) }

func must(v Version, err error) Version {
	if err != nil {
		panic(err)
	}
	return v
}

// Outdated reports whether v is from an older release cycle than Latest.
// A zero Version, one git did not report readably, is outdated.
func (v Version) Outdated() bool {
	return cmp.Or(cmp.Compare(v.Major, latest.Major), cmp.Compare(v.Minor, latest.Minor)) < 0
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// versions holds the version of every git executable that passed
// CheckVersion, by Runner.Git: git does not change while the app runs, and
// every worktree open asks.
var versions sync.Map

// CheckVersion runs "git --version" and returns *core.Error{Code: GitMissing}
// if git cannot be run. Output it cannot read gives the zero Version. A
// passed check is remembered for the process; a failed one is repeated, so
// an installed git is seen on the next call.
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
		return Version{}, core.Errorf(core.GitMissing, "git --version: %v", err)
	}
	v, _ := parseVersion(string(result.Stdout))
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
