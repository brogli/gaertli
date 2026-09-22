package git

import (
	"regexp"
	"strconv"
	"strings"
)

// percentPattern matches the first "NN%" in a progress line.
var percentPattern = regexp.MustCompile(`(\d+)%`)

// ParseProgressLine reads one stderr record from a git call run with
// --progress. ok is false when line carries no percent (a status line such
// as "Cloning into '...'"). message is line's trimmed text with a leading
// "remote: " stripped. Best-effort: progress text has no stability promise,
// so a reworded phase degrades to a status line, never a parse failure.
func ParseProgressLine(line []byte) (percent int, message string, ok bool) {
	message = strings.TrimPrefix(strings.TrimSpace(string(line)), "remote: ")
	m := percentPattern.FindStringSubmatch(message)
	if m == nil {
		return 0, message, false
	}
	percent, _ = strconv.Atoi(m[1]) // the pattern already guarantees digits only
	return percent, message, true
}
