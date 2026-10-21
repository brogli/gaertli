// Requirements: FR-UX-18, OSS-01, OSS-05

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCheckReportsOnlyDisallowedLicences(t *testing.T) {
	tests := []struct {
		name string
		deps []Dependency
		want []Dependency
	}{
		{
			name: "every licence allowed",
			deps: []Dependency{
				{Name: "a", Version: "1.0.0", Licence: "MIT"},
				{Name: "b", Version: "1.0.0", Licence: "BSD-3-Clause"},
				{Name: "c", Version: "1.0.0", Licence: "Apache-2.0"},
				{Name: "d", Version: "1.0.0", Licence: "MPL-2.0"},
			},
			want: nil,
		},
		{
			name: "one GPL-3.0 dependency among allowed ones",
			deps: []Dependency{
				{Name: "a", Version: "1.0.0", Licence: "MIT"},
				{Name: "b", Version: "1.0.0", Licence: "BSD-3-Clause"},
				{Name: "c", Version: "1.0.0", Licence: "GPL-3.0"},
				{Name: "d", Version: "1.0.0", Licence: "Apache-2.0"},
				{Name: "e", Version: "1.0.0", Licence: "MPL-2.0"},
			},
			want: []Dependency{{Name: "c", Version: "1.0.0", Licence: "GPL-3.0"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.deps)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCheckAllowsOFLForTheBundledFontsOnly(t *testing.T) {
	text := Dependency{Name: "@fontsource-variable/inter", Version: "5.3.0", Licence: "OFL-1.1"}
	mono := Dependency{Name: "@fontsource-variable/jetbrains-mono", Version: "5.3.0", Licence: "OFL-1.1"}
	other := Dependency{Name: "some-lib", Version: "1.0.0", Licence: "OFL-1.1"}

	got := Check([]Dependency{text, mono, other})

	if want := []Dependency{other}; !reflect.DeepEqual(got, want) {
		t.Errorf("Check() = %+v, want %+v", got, want)
	}
}

func TestNoticesRendersOneParagraphPerDependency(t *testing.T) {
	deps := []Dependency{
		{Name: "foo", Version: "1.2.0", Licence: "MIT", Text: "Copyright foo authors"},
		{Name: "bar", Version: "0.9.1", Licence: "BSD-3-Clause", Text: "Copyright bar authors"},
		{Name: "baz", Version: "3.0.0", Licence: "Apache-2.0", Text: "Copyright baz authors"},
	}

	out := Notices(deps)

	for _, d := range deps {
		if !strings.Contains(out, d.Name) || !strings.Contains(out, d.Version) || !strings.Contains(out, d.Licence) || !strings.Contains(out, d.Text) {
			t.Errorf("Notices() missing %+v in:\n%s", d, out)
		}
	}
	if sections := strings.Count(out, noticeSeparator) + 1; sections != len(deps) {
		t.Errorf("Notices() has %d sections, want %d", sections, len(deps))
	}
}

func TestNoticesCreditCodeAdaptedFromLazygit(t *testing.T) {
	if violations := Check(adaptedCode); len(violations) > 0 {
		t.Errorf("adapted code with a licence outside the allow-list: %v", violations)
	}
	notices := Notices(adaptedCode)
	for _, want := range []string{"github.com/jesseduffield/lazygit", "Copyright (c) 2018 Jesse Duffield", "Permission is hereby granted"} {
		if !strings.Contains(notices, want) {
			t.Errorf("notices lack %q:\n%s", want, notices)
		}
	}
}

func TestNoticesCreditTheFourDarkThemes(t *testing.T) {
	if violations := Check(themeCredits); len(violations) > 0 {
		t.Errorf("theme credits with a licence outside the allow-list: %v", violations)
	}
	notices := Notices(themeCredits)
	for _, want := range []string{
		"Copyright (c) morhetz",
		"Copyright (c) 2021 Catppuccin",
		"Copyright (c) 2016-present Sven Greb",
		"Copyright (c) 2023 Dracula Theme",
	} {
		if !strings.Contains(notices, want) {
			t.Errorf("notices lack %q:\n%s", want, notices)
		}
	}
}
