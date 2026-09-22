package git

import (
	"regexp"
	"strconv"
	"strings"
)

// percentPattern matches the first "NN%" in a progress line.
var percentPattern = regexp.MustCompile(`(\d+)%`)

// ParseProgressLine reads one stderr record from a git call run with
// --progress (StreamErr's onLine). ok is false when line carries no
// percent — git's own free-text status lines ("Cloning into '...'",
// "remote: Enumerating objects: 90, done.", a final summary) still leave
// something worth showing (the caller keeps the job's last-known percent,
// so a status line between two percent phases doesn't blank the bar). message is line's trimmed text
// with a leading "remote: " stripped (an implementation detail of the
// server-side pack step, not meaningful locally). Best-effort: unlike
// git's porcelain formats, progress text carries no stability promise, so a
// reworded phase degrades to an unparsed status line, never a parse failure.
func ParseProgressLine(line []byte) (percent int, message string, ok bool) {
	message = strings.TrimPrefix(strings.TrimSpace(string(line)), "remote: ")
	m := percentPattern.FindStringSubmatch(message)
	if m == nil {
		return 0, message, false
	}
	percent, _ = strconv.Atoi(m[1]) // the pattern already guarantees digits only
	return percent, message, true
}
