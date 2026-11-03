package editor

import (
	"errors"
	"fmt"
)

// splitCommand splits template into argv using the calling process's own
// command-line rules; no shell is invoked. windows selects the Windows
// rule: spaces and tabs separate words, double quotes group a word
// verbatim, and a backslash or single quote is an ordinary character (as
// in a folder path). The POSIX rule (windows false) also groups with
// single quotes, and a backslash makes the next character literal. Errors
// on an empty template or an unterminated quote.
func splitCommand(template string, windows bool) ([]string, error) {
	var argv []string
	var cur []byte
	inWord := false

	endWord := func() {
		if inWord {
			argv = append(argv, string(cur))
			cur = cur[:0]
			inWord = false
		}
	}

	for i := 0; i < len(template); i++ {
		switch c := template[i]; {
		case c == ' ' || c == '\t':
			endWord()
		case c == '\\' && !windows:
			if i+1 >= len(template) {
				return nil, fmt.Errorf("editor: %q: trailing backslash", template)
			}
			i++
			cur = append(cur, template[i])
			inWord = true
		case c == '"' || (c == '\'' && !windows):
			j := i + 1
			for j < len(template) && template[j] != c {
				j++
			}
			if j >= len(template) {
				return nil, fmt.Errorf("editor: %q: unterminated %c quote", template, c)
			}
			cur = append(cur, template[i+1:j]...)
			inWord = true
			i = j
		default:
			cur = append(cur, c)
			inWord = true
		}
	}
	endWord()
	if len(argv) == 0 {
		return nil, errors.New("editor: empty command")
	}
	return argv, nil
}
