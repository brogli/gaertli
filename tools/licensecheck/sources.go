package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// goReportTemplate makes `go-licenses report` print one tab-separated line
// per package: name, module version, licence, licence file.
const goReportTemplate = "{{range .}}{{.Name}}\t{{.Version}}\t{{.LicenseName}}\t{{.LicensePath}}\n{{end}}"

// parseGoLicenses reads goReportTemplate's output and drops ownModule.
func parseGoLicenses(report []byte, ownModule string) ([]Dependency, error) {
	var deps []Dependency
	for line := range strings.Lines(string(report)) {
		f := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("go-licenses report: want 4 fields, got %q", line)
		}
		if f[0] == ownModule {
			continue
		}
		deps = append(deps, Dependency{Name: f[0], Version: f[1], Licence: f[2], File: f[3]})
	}
	return deps, nil
}

// parsePnpmLicenses reads `pnpm licenses list --json`: licence → packages,
// each with its installed versions and their directories. Sorted by name,
// then version.
func parsePnpmLicenses(report []byte) ([]Dependency, error) {
	var byLicence map[string][]struct {
		Name     string   `json:"name"`
		Versions []string `json:"versions"`
		Paths    []string `json:"paths"`
		License  string   `json:"license"`
	}
	if err := json.Unmarshal(report, &byLicence); err != nil {
		return nil, fmt.Errorf("pnpm licenses list: %w", err)
	}
	var deps []Dependency
	for _, pkgs := range byLicence {
		for _, p := range pkgs {
			if len(p.Paths) != len(p.Versions) {
				return nil, fmt.Errorf("pnpm licenses list: %s has %d versions but %d paths", p.Name, len(p.Versions), len(p.Paths))
			}
			for i, v := range p.Versions {
				deps = append(deps, Dependency{Name: p.Name, Version: v, Licence: p.License, File: p.Paths[i]})
			}
		}
	}
	slices.SortFunc(deps, func(a, b Dependency) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Version, b.Version))
	})
	return deps, nil
}

// licenceFileNames are the names a package directory's licence file goes by.
var licenceFileNames = []string{"LICENSE*", "LICENCE*", "license*", "licence*", "COPYING*"}

// sameLicenceAs names packages published without a licence file whose
// licence text is another listed dependency's: @wailsio/runtime is built
// from the Wails repository under its MIT licence.
var sameLicenceAs = map[string]string{
	"@wailsio/runtime": "github.com/wailsapp/wails/v3",
}

// loadTexts fills each dependency's Text from File: the licence file itself,
// or the first licence file in that package directory, else the text of the
// dependency sameLicenceAs names. A dependency with none of these is an
// error: its notice would be incomplete.
func loadTexts(deps []Dependency) ([]Dependency, error) {
	out := slices.Clone(deps)
	byName := map[string]string{}
	for i, d := range out {
		if source, ok := sameLicenceAs[d.Name]; ok && byName[source] != "" {
			out[i].Text = byName[source]
			continue
		}
		path, err := licenceFile(d.File)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", d.Name, d.Version, err)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", d.Name, d.Version, err)
		}
		out[i].Text = string(bytes.TrimSpace(text))
		byName[d.Name] = out[i].Text
	}
	return out, nil
}

func licenceFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return path, nil
	}
	for _, pattern := range licenceFileNames {
		if matches, _ := filepath.Glob(filepath.Join(path, pattern)); len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("no licence file in %s", path)
}
