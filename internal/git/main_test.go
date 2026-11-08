// TestMain lives in the external test package: gittest imports package git,
// so a TestMain inside package git that called gittest.Main would cycle.
package git_test

import (
	"testing"

	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/testbin"
)

func TestMain(m *testing.M) {
	gittest.Main(m, testbin.Bin{Path: git.FakegitPath, Pkg: "./testdata/fakegit"})
}
