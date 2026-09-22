package remote

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf16"
)

// wslPresent reports whether wsl.exe is on PATH (mirrors
// internal/app/prereq.go's gitPresent()). Gates every WSL action.
func wslPresent() bool {
	_, err := exec.LookPath("wsl.exe")
	return err == nil
}

// parseDistroList is the pure parser for `wsl --list --verbose`'s output
// (Research used: raw may be UTF-16LE when wsl.exe's stdout is piped).
// Drops the header line by position (not by matching localized column
// text), and reads each row from the end — last field VERSION (a digit,
// locale-safe), second-last STATE (locale text, kept opaque, not used for
// any decision), everything before that re-joined as NAME (so a
// space-containing distro name parses correctly). A leading '*' marks
// Default.
func parseDistroList(raw []byte) ([]Distro, error) {
	lines := strings.Split(decodeWSLText(raw), "\n")
	var rows []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			rows = append(rows, line)
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	rows = rows[1:] // drop the header

	distros := make([]Distro, 0, len(rows))
	for _, row := range rows {
		d, err := parseDistroRow(row)
		if err != nil {
			return nil, err
		}
		distros = append(distros, d)
	}
	return distros, nil
}

func parseDistroRow(row string) (Distro, error) {
	fields := strings.Fields(row)
	var isDefault bool
	if len(fields) > 0 && fields[0] == "*" {
		isDefault, fields = true, fields[1:]
	}
	if len(fields) < 3 {
		return Distro{}, fmt.Errorf("parseDistroList: bad row %q", row)
	}

	version, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return Distro{}, fmt.Errorf("parseDistroList: bad row %q: %w", row, err)
	}
	name := strings.Join(fields[:len(fields)-2], " ")
	return Distro{Name: name, Version: version, Default: isDefault}, nil
}

// decodeWSLText undoes wsl.exe's own UTF-16LE-when-piped behavior (Research
// used, unverified — testing.md's manual row asks for a real capture to
// confirm this heuristic): a BOM if present, else the "every other byte is
// 0x00" pattern, else the bytes are already UTF-8/ASCII.
func decodeWSLText(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		return utf16LEToString(raw[2:])
	}
	if looksUTF16LE(raw) {
		return utf16LEToString(raw)
	}
	return string(raw)
}

// looksUTF16LE applies the NUL-byte-pattern heuristic (Research used, Gap):
// ASCII text encoded as UTF-16LE has a 0x00 byte at every odd index.
func looksUTF16LE(b []byte) bool {
	if len(b) < 2 || len(b)%2 != 0 {
		return false
	}
	for i := 1; i < len(b); i += 2 {
		if b[i] != 0x00 {
			return false
		}
	}
	return true
}

func utf16LEToString(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1] // drop a stray trailing byte rather than panic
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u16))
}
