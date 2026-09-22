package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
)

// Version is git's own version, as reported by "git --version".
type Version struct{ Major, Minor, Patch int }

// MinVersion is the oldest supported git.
var MinVersion = Version{Major: 2, Minor: 40, Patch: 0}

// Less reports whether v is older than o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// CheckVersion runs "git --version" and returns *core.Error{Code: GitTooOld}
// if git cannot be started, its output cannot be parsed, or it is older
// than MinVersion.
func (r Runner) CheckVersion(ctx context.Context) (Version, error) {
	result, err := r.Run(ctx, Call{Args: []string{"--version"}, ReadOnly: true})
	if err != nil {
		return Version{}, core.Errorf(core.GitTooOld, "git --version: %v", err)
	}
	v, err := parseVersion(string(result.Stdout))
	if err != nil {
		return Version{}, core.Errorf(core.GitTooOld, "%v", err)
	}
	if v.Less(MinVersion) {
		return Version{}, core.Errorf(core.GitTooOld, "git %s is older than the minimum %s", v, MinVersion)
	}
	return v, nil
}

// parseVersion extracts Major.Minor.Patch from the first three
// dot-separated numeric groups of "git --version"'s stdout, e.g.
// "git version 2.43.0" or "git version 2.43.0.windows.1".
func parseVersion(out string) (Version, error) {
	for _, field := range strings.Fields(out) {
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
