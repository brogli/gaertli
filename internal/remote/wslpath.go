package remote

import "strings"

// Distro is one installed WSL distro (FR-REPO-07's picker), bound to the
// frontend by AppService.ListWSLDistros.
type Distro struct {
	Name    string `json:"name"`
	Version int    `json:"version"` // 1 or 2; ADR-3: WSL 1 is out of scope
	Default bool   `json:"default"`
}

// wslPrefixes are the two UNC forms a WSL path is found under (Research
// used: neither is documented as deprecated, so both are accepted).
var wslPrefixes = []string{`\\wsl.localhost\`, `\\wsl$\`}

// ClassifyWSLPath reports whether path names a location inside a WSL
// distro — \\wsl.localhost\<distro>\... or the older \\wsl$\<distro>\...,
// both accepted — and, if so, the distro name as given (distro names are
// case-sensitive to wsl.exe) and the POSIX path inside it. The UNC prefix
// itself and the backslash separators are matched case-insensitively
// (Windows path convention). false if path has no such prefix, or names the
// distro root with no folder inside it (FR-REPO-07 always opens a folder,
// never a distro's own root).
func ClassifyWSLPath(path string) (distro, linuxPath string, ok bool) {
	rest, ok := trimWSLPrefix(path)
	if !ok {
		return "", "", false
	}
	distro, tail, found := strings.Cut(rest, `\`)
	if !found || tail == "" {
		return "", "", false
	}
	return distro, "/" + strings.ReplaceAll(tail, `\`, "/"), true
}

// BuildWSLPath is ClassifyWSLPath's inverse: the \\wsl.localhost\<distro>\...
// UNC form of a distro and a POSIX path inside it — the same form
// AppService.OpenPath already produces and the one spec 04's single-instance
// launch path already classifies correctly. Pure, no I/O.
func BuildWSLPath(distro, linuxPath string) string {
	return `\\wsl.localhost\` + distro + strings.ReplaceAll(linuxPath, "/", `\`)
}

func trimWSLPrefix(path string) (string, bool) {
	for _, prefix := range wslPrefixes {
		if len(path) >= len(prefix) && strings.EqualFold(path[:len(prefix)], prefix) {
			return path[len(prefix):], true
		}
	}
	return "", false
}

// windowsPathToMount converts an absolute Windows path (os.Executable()'s
// own form) to its default WSL DrvFs mount form: Windows drives mount under
// /mnt/<lowercase-letter>/ by default (a customised wsl.conf [automount]
// root breaks this, the same category of default-config assumption
// ClassifyWSLPath already makes for the UNC prefix form). ok=false for
// anything not of the form "<letter>:\...".
func windowsPathToMount(winPath string) (mountPath string, ok bool) {
	if len(winPath) < 3 || winPath[1] != ':' || winPath[2] != '\\' {
		return "", false
	}
	letter := winPath[0]
	if (letter < 'a' || letter > 'z') && (letter < 'A' || letter > 'Z') {
		return "", false
	}
	rest := strings.ReplaceAll(winPath[3:], `\`, "/")
	return "/mnt/" + strings.ToLower(string(letter)) + "/" + rest, true
}
