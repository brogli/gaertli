package editor

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/testbin"
)

// argvWriterBin writes its own argv to a file next to itself, standing in
// for a real editor.
var argvWriterBin string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &argvWriterBin, Pkg: "./testdata/argvwriter"})
}

func TestFR_ED_05_Default(t *testing.T) {
	want := core.EditorProfile{Name: "the editor", FolderCmd: "editor {path}", FileCmd: "editor {path}", FileAtLineCmd: "editor {path}:{line}"}
	if got := Default(); got != want {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestFR_ED_04_FillPlaceholders(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		path string
		line int
		want []string
	}{
		{"path only, no line", []string{"editor", "{path}"}, "/a/b", 0, []string{"editor", "/a/b"}},
		{"path and line", []string{"editor", "{path}:{line}"}, "/a/b", 42, []string{"editor", "/a/b:42"}},
		{"line token left literal when line is 0", []string{"cmd", "{line}"}, "/a/b", 0, []string{"cmd", "{line}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fillPlaceholders(tt.argv, tt.path, tt.line)
			if !slices.Equal(got, tt.want) {
				t.Errorf("fillPlaceholders(%v, %q, %d) = %v, want %v", tt.argv, tt.path, tt.line, got, tt.want)
			}
		})
	}
}

func TestFR_ED_04_BuildCommandViaOpenFileSelectsTheRightTemplate(t *testing.T) {
	tests := []struct {
		name    string
		profile core.EditorProfile
		line    int
		want    []string
	}{
		{
			name:    "line 0 uses FileCmd",
			profile: core.EditorProfile{FileCmd: "editor {path}", FileAtLineCmd: "editor {path}:{line}"},
			line:    0,
			want:    []string{"editor", "/a/b"},
		},
		{
			name:    "line > 0 uses FileAtLineCmd",
			profile: core.EditorProfile{FileCmd: "editor {path}", FileAtLineCmd: "editor {path}:{line}"},
			line:    42,
			want:    []string{"editor", "/a/b:42"},
		},
		{
			name:    "no FileAtLineCmd falls back to FileCmd",
			profile: core.EditorProfile{FileCmd: "editor {path}"},
			line:    42,
			want:    []string{"editor", "/a/b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argv, err := buildCommand(fileTemplate(tt.profile, tt.line), "/a/b", tt.line)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(argv, tt.want) {
				t.Errorf("argv = %v, want %v", argv, tt.want)
			}
		})
	}
}

func TestFR_ED_06_OpenFolderReturnsEditorNotFoundWhenNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty: nothing resolves
	profile := core.EditorProfile{FolderCmd: "definitely-not-a-real-editor {path}"}

	err := OpenFolder(profile, "/tmp/x")

	coretest.AssertCode(t, err, core.EditorNotFound)
}

func TestFR_ED_01_OpenFolderStartsTheConfiguredCommandDetached(t *testing.T) {
	profile := core.EditorProfile{FolderCmd: argvWriterBin + " {path}"}
	outFile := argvWriterBin + ".out"
	t.Cleanup(func() { _ = os.Remove(outFile) })

	start := time.Now()
	if err := OpenFolder(profile, "/tmp/x"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("OpenFolder took %v, want promptly (detached, not waiting on the editor)", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	var content []byte
	var err error
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
	if got := string(content); got != "/tmp/x" {
		t.Errorf("helper argv = %q, want %q", got, "/tmp/x")
	}
}
