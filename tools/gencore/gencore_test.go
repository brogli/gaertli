// Requirements: QA-PORT-02

package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGeneratesFacadeClientDispatcher(t *testing.T) {
	files, err := Generate("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range outputs {
		golden := filepath.Join("testdata/fixture/golden", filepath.Base(out.path)+".golden")
		if *update {
			if err := os.WriteFile(golden, files[out.path], 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if string(files[out.path]) != string(want) {
			t.Errorf("%s differs from golden (run with -update to accept):\n%s", golden, files[out.path])
		}
	}
}

func TestRejectsNonJSONSignatures(t *testing.T) {
	cases := map[string]string{
		"noctx":      "Version: first parameter must be context.Context",
		"chanparam":  "Stream: parameter out: chan string is not JSON-serialisable",
		"funcresult": "Hook: result: func() is not JSON-serialisable",
		"ifacefield": "Load: parameter o: interface{Match(string) bool} is not JSON-serialisable",
		"noerror":    "Ping: last result must be error",
		"unnamed":    "Load: parameter 2 has no name",
	}
	for dir, want := range cases {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			files, err := Generate(filepath.Join("testdata/bad", dir))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want containing %q", err, want)
			}
			if files != nil {
				t.Error("files returned with error")
			}
		})
	}
}

func TestGeneratedCodeIsCurrent(t *testing.T) {
	files, err := Generate("../../internal/core")
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range outputs {
		want, err := os.ReadFile(filepath.Join("../..", out.path))
		if err != nil {
			t.Fatal(err)
		}
		if string(files[out.path]) != string(want) {
			t.Errorf("%s is stale; run go generate ./internal/core", out.path)
		}
	}
}
