package remote

import (
	"encoding/binary"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf16"
)

// wslPresent reports whether wsl.exe is on PATH.
func wslPresent() bool {
	_, err := exec.LookPath("wsl.exe")
	return err == nil
}

// distrosOf lists the distros `wsl --list --quiet` names, one per line,
// each with the WSL version `wsl --list --verbose` prints on the row that
// starts with its name: the row's last field. Nothing else of the verbose
// table is read, so its localized titles and states, the default marker
// and any other line do not matter. A name with no such row, or no number
// there, has Version 0. Both outputs are UTF-16LE when piped from an old
// wsl.exe.
func distrosOf(quiet, verbose []byte) []Distro {
	rows := lines(verbose)
	var distros []Distro
	for _, name := range lines(quiet) {
		distros = append(distros, Distro{Name: name, Version: versionOf(name, rows)})
	}
	return distros
}

// versionOf is the last field of the first of rows that holds name and then
// more, 0 if none or not a number.
func versionOf(name string, rows []string) int {
	for _, row := range rows {
		rest, ok := strings.CutPrefix(strings.TrimLeft(row, "* "), name)
		fields := strings.Fields(rest)
		if ok && len(fields) > 0 && strings.TrimLeft(rest, " \t") != rest {
			version, _ := strconv.Atoi(fields[len(fields)-1])
			return version
		}
	}
	return 0
}

// lines is raw's non-blank lines, trimmed.
func lines(raw []byte) []string {
	var out []string
	for line := range strings.Lines(decodeWSLText(raw)) {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// decodeWSLText undoes wsl.exe's own UTF-16LE-when-piped behaviour: a BOM
// if present, else the "every other byte is 0x00" pattern, else the bytes
// are already UTF-8/ASCII.
func decodeWSLText(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		return utf16LEToString(raw[2:])
	}
	if looksUTF16LE(raw) {
		return utf16LEToString(raw)
	}
	return string(raw)
}

// looksUTF16LE applies the NUL-byte-pattern heuristic: ASCII text encoded
// as UTF-16LE has a 0x00 byte at every odd index.
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
