package remote

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf16"
)

// quiet and verbose are what `wsl --list --quiet` and `--verbose` print
// for the same two distros.
const (
	quiet   = "Ubuntu-22.04\r\nDebian\r\n"
	verbose = "  NAME               STATE           VERSION\r\n" +
		"* Ubuntu-22.04       Running         2\r\n" +
		"  Debian             Stopped         1\r\n"
)

var wantDistros = []Distro{
	{Name: "Ubuntu-22.04", Version: 2},
	{Name: "Debian", Version: 1},
}

func TestQA_PORT_02_DistrosOfNamesEachDistroWithItsWSLVersion(t *testing.T) {
	tests := []struct {
		name, quiet, verbose string
		want                 []Distro
	}{
		{"the plain table", quiet, verbose, wantDistros},
		{
			"localized titles, a state with a space, a first row without a readable version, stray lines around the table",
			"Odd\r\nUbuntu\r\nDebian\r\n",
			"Some notice from wsl.exe\r\n" +
				"  NAME      ZUSTAND            VERSION\r\n" +
				"* Odd       Wird ausgeführt    ?\r\n" +
				"  Ubuntu    Wird ausgeführt    2\r\n" +
				"  Debian    Beendet            1\r\n" +
				"wsl: a notice below the table 2\r\n",
			[]Distro{{Name: "Odd"}, {Name: "Ubuntu", Version: 2}, {Name: "Debian", Version: 1}},
		},
		{
			"a name that begins another",
			"Ubuntu\r\nUbuntu-22.04\r\n",
			"  NAME           STATE      VERSION\r\n" +
				"  Ubuntu-22.04   Running    2\r\n" +
				"* Ubuntu         Stopped    1\r\n",
			[]Distro{{Name: "Ubuntu", Version: 1}, {Name: "Ubuntu-22.04", Version: 2}},
		},
		{"no versions to read", quiet, "", []Distro{{Name: "Ubuntu-22.04"}, {Name: "Debian"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := distrosOf([]byte(tt.quiet), []byte(tt.verbose)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("distrosOf = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFR_REPO_07_DecodeWSLTextUTF16LE(t *testing.T) {
	for _, bom := range []bool{true, false} {
		got := distrosOf(utf16LEBytes(quiet, bom), utf16LEBytes(verbose, bom))
		if !reflect.DeepEqual(got, wantDistros) {
			t.Errorf("BOM %v: distrosOf = %+v, want %+v", bom, got, wantDistros)
		}
	}
}

func utf16LEBytes(s string, bom bool) []byte {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 0, len(units)*2+2)
	if bom {
		buf = append(buf, 0xFF, 0xFE)
	}
	for _, u := range units {
		buf = binary.LittleEndian.AppendUint16(buf, u)
	}
	return buf
}

func TestQA_PORT_02_WslPresentFalseWhenNotOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if wslPresent() {
		t.Error("wslPresent() = true, want false")
	}
}
