// Command licensecheck checks the licence of every dependency shipped in the
// app against an allow-list and writes THIRD-PARTY-NOTICES.txt.
package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// AllowedLicences are the SPDX IDs of the permissive licences a shipped
// dependency may carry.
var AllowedLicences = []string{"MIT", "BSD-2-Clause", "BSD-3-Clause", "ISC", "0BSD", "Apache-2.0", "MPL-2.0"}

// Dependency is one Go package or JS package with its licence. File is the
// licence file, or the package directory to search for one; Text is its
// content, filled by loadTexts.
type Dependency struct {
	Name, Version, Licence, File, Text string
}

// Check reports every dependency whose licence isn't in AllowedLicences.
func Check(deps []Dependency) (violations []Dependency) {
	for _, d := range deps {
		if !slices.Contains(AllowedLicences, d.Licence) {
			violations = append(violations, d)
		}
	}
	return violations
}

// noticeSeparator divides one dependency's section from the next.
var noticeSeparator = "\n\n" + strings.Repeat("-", 72) + "\n\n"

// Notices renders deps as THIRD-PARTY-NOTICES.txt: per dependency, sorted
// by name, a "name version / licence" header and the licence text.
func Notices(deps []Dependency) string {
	sorted := slices.Clone(deps)
	slices.SortFunc(sorted, func(a, b Dependency) int { return cmp.Compare(a.Name, b.Name) })

	paragraphs := make([]string, len(sorted))
	for i, d := range sorted {
		paragraphs[i] = fmt.Sprintf("%s %s\n%s\n\n%s", d.Name, d.Version, d.Licence, strings.TrimSpace(d.Text))
	}
	return strings.Join(paragraphs, noticeSeparator) + "\n"
}
