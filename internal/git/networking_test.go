package git

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// git itself talks to the network; this package only shells out to it.
func TestQA_SEC_01_NoNetworkImports(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if importPath == "net" || importPath == "net/http" {
				t.Errorf("%s imports %q", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
