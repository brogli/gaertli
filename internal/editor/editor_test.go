// Requirements: FR-ED-01, FR-ED-04, FR-ED-06, FR-UX-02, FR-UX-11

package editor

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/testbin"
	"github.com/brogli/gaertli/internal/testwait"
)

// argvWriterBin writes its argv to the file its first argument names,
// standing in for a real editor.
var argvWriterBin string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &argvWriterBin, Pkg: "./testdata/argvwriter"})
}

func TestFillPlaceholders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		argv []string
		path string
		line int
		want []string
	}{
		{"path only, no line", []string{"myed", "{path}"}, "/a/b", 0, []string{"myed", "/a/b"}},
		{"path and line", []string{"myed", "{path}:{line}"}, "/a/b", 42, []string{"myed", "/a/b:42"}},
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

func TestBuildCommandViaOpenFileSelectsTheRightTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		profile core.EditorProfile
		line    int
		want    []string
	}{
		{
			name:    "line 0 uses FileCmd",
			profile: core.EditorProfile{FileCmd: "myed {path}", FileAtLineCmd: "myed {path}:{line}"},
			line:    0,
			want:    []string{"myed", "/a/b"},
		},
		{
			name:    "line > 0 uses FileAtLineCmd",
			profile: core.EditorProfile{FileCmd: "myed {path}", FileAtLineCmd: "myed {path}:{line}"},
			line:    42,
			want:    []string{"myed", "/a/b:42"},
		},
		{
			name:    "no FileAtLineCmd falls back to FileCmd",
			profile: core.EditorProfile{FileCmd: "myed {path}"},
			line:    42,
			want:    []string{"myed", "/a/b"},
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

func TestOpenFolderReturnsEditorNotFoundWhenNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty: nothing resolves
	profile := core.EditorProfile{FolderCmd: "definitely-not-a-real-editor {path}"}

	_, err := OpenFolder(profile, "/tmp/x")

	coretest.AssertCode(t, err, core.EditorNotFound)
}

func TestOpenFolderNamesTheFailedCommandInArgs(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty: nothing resolves
	profile := core.EditorProfile{FolderCmd: "definitely-not-a-real-editor {path}"}

	_, err := OpenFolder(profile, "/tmp/x")

	ce := coretest.AsError(t, err, core.EditorNotFound)
	if want := "definitely-not-a-real-editor"; ce.Args["command"] != want {
		t.Errorf("Args[command] = %q, want %q", ce.Args["command"], want)
	}
	if ce.Key != "editorNotOnPath" {
		t.Errorf("Key = %q, want editorNotOnPath", ce.Key)
	}
}

func TestOpenFolderReturnsWhileTheEditorIsStillRunning(t *testing.T) {
	t.Parallel()
	cmd, outFile := argvWriterCmd(t)
	hold := outFile + ".hold"
	if err := os.WriteFile(hold, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	profile := core.EditorProfile{FolderCmd: cmd}

	if _, err := OpenFolder(profile, "/tmp/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outFile + ".exited"); err == nil {
		t.Error("OpenFolder returned only after the editor exited, want it detached")
	}

	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	awaitFile(t, outFile+".exited")
	if got := awaitFile(t, outFile); got != "/tmp/x" {
		t.Errorf("helper argv = %q, want %q", got, "/tmp/x")
	}
}

func TestOpenFileReturnsTheResolvedArgv(t *testing.T) {
	t.Parallel()
	cmd, outFile := argvWriterCmd(t)
	profile := core.EditorProfile{FileCmd: cmd}

	argv, err := OpenFile(profile, "/tmp/x", 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{argvWriterBin, outFile, "/tmp/x"}; !slices.Equal(argv, want) {
		t.Errorf("argv = %v, want %v", argv, want)
	}
	awaitFile(t, outFile)
}

func TestALauncherThatExitsNonZeroStillCountsAsStarted(t *testing.T) {
	t.Parallel()
	cmd, outFile := argvWriterCmd(t)
	profile := core.EditorProfile{FileCmd: cmd}

	if _, err := OpenFile(profile, "/tmp/x", 0); err != nil {
		t.Fatalf("OpenFile = %v, want nil: argvwriter exits 1 like explorer.exe after a successful open", err)
	}
	awaitFile(t, outFile)
}

// argvWriterCmd returns a command template that runs argvWriterBin on
// {path}, and the file it writes its argv to.
func argvWriterCmd(t *testing.T) (template, outFile string) {
	t.Helper()
	outFile = filepath.Join(t.TempDir(), "argv")
	return argvWriterBin + " " + outFile + " {path}", outFile
}

// awaitFile waits for the helper to write path and returns its content.
func awaitFile(t *testing.T, path string) string {
	t.Helper()
	var content []byte
	testwait.Until(func() bool {
		var err error
		content, err = os.ReadFile(path)
		return err == nil
	})
	return string(content)
}
