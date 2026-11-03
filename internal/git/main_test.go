// TestMain lives in the external test package: gittest imports package git,
// so a TestMain inside package git that called gittest.Main would cycle.
package git_test

import (
	"testing"

	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/testbin"
)

func TestMain(m *testing.M) {
	gittest.Main(m, testbin.Bin{Path: git.FakegitPath, Pkg: "./testdata/fakegit"})
}
