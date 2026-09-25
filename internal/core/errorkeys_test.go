package core_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
)

// errorKeyLiteral matches an ErrorfArgs or Retag call's key literal: a
// code, then a quoted key, as the first two arguments after "core.".
var errorKeyLiteral = regexp.MustCompile(`\b(?:core\.ErrorfArgs|\w+\.Retag)\(\s*core\.\w+,\s*"([A-Za-z0-9]+)"`)

// walkGoFiles calls fn with the contents of every .go file under this
// module, skipping node_modules and hidden directories (.git, .direnv,
// agent worktrees under .claude).
func walkGoFiles(t *testing.T, fn func(path, source string)) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(path, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestFR_UX_11_EveryErrorKeyLiteralIsInTheTable greps every .go file for
// ErrorfArgs and Retag call sites, so it catches a key typo or a stale
// table entry without having to exercise every site.
func TestFR_UX_11_EveryErrorKeyLiteralIsInTheTable(t *testing.T) {
	used := map[string]bool{}
	walkGoFiles(t, func(path, source string) {
		for _, m := range errorKeyLiteral.FindAllStringSubmatch(source, -1) {
			used[m[1]] = true
		}
	})
	if len(used) == 0 {
		t.Fatal("found no core.ErrorfArgs/Retag call sites; the regexp may be stale")
	}

	for key := range used {
		if _, ok := core.ErrorKeys[key]; !ok {
			t.Errorf("key %q is used in an ErrorfArgs/Retag call but missing from core.ErrorKeys", key)
		}
	}
	for key := range core.ErrorKeys {
		if !used[key] {
			t.Errorf("core.ErrorKeys[%q] has no ErrorfArgs/Retag call site using it", key)
		}
	}
}

// TestFR_UX_11_EveryErrorKeyHasACatalogueEntryWithItsArgs fails if the
// frontend's en.json lacks an errors.<key> entry for a key of ErrorKeys, or
// the entry's {placeholders} are not exactly that key's args.
func TestFR_UX_11_EveryErrorKeyHasACatalogueEntryWithItsArgs(t *testing.T) {
	data, err := os.ReadFile("../../frontend/src/locales/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var en struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(data, &en); err != nil {
		t.Fatal(err)
	}

	placeholder := regexp.MustCompile(`\{(\w+)\}`)
	for key, args := range core.ErrorKeys {
		text, ok := en.Errors[key]
		if !ok {
			t.Errorf("en.json has no errors.%s", key)
			continue
		}
		var used []string
		for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
			used = append(used, m[1])
		}
		slices.Sort(used)
		used = slices.Compact(used)
		if want := slices.Sorted(slices.Values(args)); !slices.Equal(used, want) {
			t.Errorf("errors.%s uses {%v}, want its args {%v}", key, used, want)
		}
	}
}

// TestFR_UX_11_NoDirectMessageOrKeyAssignmentOutsideCore fails if code
// outside this package assigns .Message or .Key on some *core.Error
// directly (ce.Message = ...): every such write must go through
// core.ErrorfArgs or (*core.Error).Retag, so a message and its key never
// drift apart.
func TestFR_UX_11_NoDirectMessageOrKeyAssignmentOutsideCore(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	corePkg := filepath.Join(root, "internal", "core")

	checked := 0
	walkGoFiles(t, func(path, source string) {
		if strings.HasPrefix(path, corePkg) {
			return // Retag and ErrorfArgs live here
		}
		checked++
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				// Only a plain ident.Field selector: a nested field such as
				// a job's e.job.Message is not a core.Error's.
				if _, isIdent := sel.X.(*ast.Ident); !isIdent {
					continue
				}
				if sel.Sel.Name == "Message" || sel.Sel.Name == "Key" {
					t.Errorf("%s: assigns .%s directly; use (*core.Error).Retag or core.ErrorfArgs", fset.Position(lhs.Pos()), sel.Sel.Name)
				}
			}
			return true
		})
	})
	if checked == 0 {
		t.Fatal("checked no files outside internal/core; the path filter may be stale")
	}
}
