package remote

import "strings"

// Distro is one installed WSL distro.
type Distro struct {
	Name    string
	Version int // the WSL version it runs on: 1 or 2; 0 when wsl.exe did not say
}

// wslPrefixes are the two UNC forms a WSL path is found under; neither is
// documented as deprecated, so both are accepted.
var wslPrefixes = []string{`\\wsl.localhost\`, `\\wsl$\`}

// ClassifyWSLPath reports whether path names a folder inside a WSL distro
// (\\wsl.localhost\<distro>\... or \\wsl$\<distro>\...) and, if so, the
// distro name as given (case-sensitive to wsl.exe) and the POSIX path
// inside it. The UNC prefix is matched case-insensitively. A distro root
// with no folder inside it is not a match.
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
// UNC form of a distro and a POSIX path inside it.
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
// /mnt/<lowercase-letter>/ unless wsl.conf's [automount] root is
// customised. ok=false for anything not of the form "<letter>:\...".
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
