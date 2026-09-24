package remote

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf16"
)

const canned = "  NAME               STATE           VERSION\r\n" +
	"* Ubuntu-22.04       Running         2\r\n" +
	"  My Distro          Stopped         1\r\n"

var wantDistros = []Distro{
	{Name: "Ubuntu-22.04", Version: 2},
	{Name: "My Distro", Version: 1},
}

func TestFR_REPO_07_ParseDistroListUTF8(t *testing.T) {
	got, err := parseDistroList([]byte(canned))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantDistros) {
		t.Errorf("parseDistroList = %+v, want %+v", got, wantDistros)
	}
}

func TestFR_REPO_07_DecodeWSLTextUTF16LE(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"with BOM", utf16LEBytes(canned, true)},
		{"without BOM", utf16LEBytes(canned, false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDistroList(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, wantDistros) {
				t.Errorf("parseDistroList(decodeWSLText(...)) = %+v, want %+v", got, wantDistros)
			}
		})
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
