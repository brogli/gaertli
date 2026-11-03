// Requirements: OSS-01

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGoLicensesReadsTheReportAndSkipsOwnModule(t *testing.T) {
	report := []byte("github.com/adrg/xdg\tv0.5.3\tMIT\t/mod/xdg/LICENSE\n" +
		"github.com/brogli/nebegit\t\tBSD-3-Clause\t/repo/LICENSE\n" +
		"golang.org/x/sys/unix\tv0.48.0\tBSD-3-Clause\t/mod/sys/LICENSE\n")

	got, err := parseGoLicenses(report, "github.com/brogli/nebegit")
	if err != nil {
		t.Fatal(err)
	}
	want := []Dependency{
		{Name: "github.com/adrg/xdg", Version: "v0.5.3", Licence: "MIT", File: "/mod/xdg/LICENSE"},
		{Name: "golang.org/x/sys/unix", Version: "v0.48.0", Licence: "BSD-3-Clause", File: "/mod/sys/LICENSE"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseGoLicenses() = %+v, want %+v", got, want)
	}
}

func TestParsePnpmLicensesFlattensEveryVersion(t *testing.T) {
	report := []byte(`{
  "MIT": [{"name": "vue", "versions": ["3.5.42"], "paths": ["/nm/vue"], "license": "MIT"}],
  "ISC": [{"name": "picocolors", "versions": ["1.1.0", "1.1.1"], "paths": ["/nm/pc0", "/nm/pc1"], "license": "ISC"}]
}`)

	got, err := parsePnpmLicenses(report)
	if err != nil {
		t.Fatal(err)
	}
	want := []Dependency{
		{Name: "picocolors", Version: "1.1.0", Licence: "ISC", File: "/nm/pc0"},
		{Name: "picocolors", Version: "1.1.1", Licence: "ISC", File: "/nm/pc1"},
		{Name: "vue", Version: "3.5.42", Licence: "MIT", File: "/nm/vue"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePnpmLicenses() = %+v, want %+v", got, want)
	}
}

func TestLoadTextsReadsALicenceFileOrFindsOneInAPackageDir(t *testing.T) {
	dir := t.TempDir()
	goLicence := filepath.Join(dir, "LICENSE")
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.WriteFile(goLicence, []byte("MIT text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "LICENSE.md"), []byte("ISC text"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps, err := loadTexts([]Dependency{{Name: "a", File: goLicence}, {Name: "b", File: pkgDir}})
	if err != nil {
		t.Fatal(err)
	}
	if deps[0].Text != "MIT text" || deps[1].Text != "ISC text" {
		t.Errorf("texts = %q, %q", deps[0].Text, deps[1].Text)
	}
}

func TestLoadTextsFailsWhenAPackageHasNoLicenceFile(t *testing.T) {
	_, err := loadTexts([]Dependency{{Name: "nolicence", File: t.TempDir()}})

	if err == nil || !strings.Contains(err.Error(), "nolicence") {
		t.Errorf("err = %v, want one naming the package", err)
	}
}

func TestLoadTextsUsesTheEmbeddedTextForAKnownPackageWithoutOne(t *testing.T) {
	restore := noLicenceFileText
	noLicenceFileText = map[string]string{"pkg-without-licence": "MIT, embedded"}
	t.Cleanup(func() { noLicenceFileText = restore })

	deps, err := loadTexts([]Dependency{{Name: "pkg-without-licence", File: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if deps[0].Text != "MIT, embedded" {
		t.Errorf("text = %q", deps[0].Text)
	}
}

func TestLoadTextsUsesTheSameLicenceTextForAPackageWithoutOne(t *testing.T) {
	licence := filepath.Join(t.TempDir(), "LICENSE")
	if err := os.WriteFile(licence, []byte("MIT, Wails authors"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps, err := loadTexts([]Dependency{
		{Name: "github.com/wailsapp/wails/v3", File: licence},
		{Name: "@wailsio/runtime", File: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deps[1].Text != "MIT, Wails authors" {
		t.Errorf("@wailsio/runtime text = %q", deps[1].Text)
	}
}
