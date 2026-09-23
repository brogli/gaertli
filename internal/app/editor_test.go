package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/services"
	"github.com/brogli/nebegit/internal/testbin"
)

// argvWriterBin writes its own argv to a file next to itself, standing in
// for a real editor (internal/editor's own test fixture, reused here).
var argvWriterBin string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &argvWriterBin, Pkg: "../editor/testdata/argvwriter"})
}

func TestFR_ED_08_TestEditorProfileRunsTheLocalCoreDirectlyAgainstTheHomeDirectory(t *testing.T) {
	reg, _ := newTestRegistry()
	reg.SetLocal(services.New(func(core.Event) {}))
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}
	outFile := argvWriterBin + ".out"
	t.Cleanup(func() { _ = os.Remove(outFile) })

	profile := core.EditorProfile{FolderCmd: argvWriterBin + " {path}"}
	if err := s.TestEditorProfile(context.Background(), "", profile); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var content []byte
	for time.Now().Before(deadline) {
		content, err = os.ReadFile(outFile)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("helper never wrote its argv file")
	}
	if got := string(content); got != home {
		t.Errorf("helper argv = %q, want the home directory %q", got, home)
	}
}
