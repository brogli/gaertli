package editor

import (
	"errors"
	"fmt"
)

// splitCommand splits template into argv using POSIX-style shell word
// splitting (quotes, backslash escapes); no shell is invoked. Errors on an
// empty template or an unterminated quote.
func splitCommand(template string) ([]string, error) {
	var argv []string
	var cur []byte
	inWord := false

	for i := 0; i < len(template); i++ {
		switch c := template[i]; c {
		case ' ', '\t':
			if inWord {
				argv = append(argv, string(cur))
				cur = cur[:0]
				inWord = false
			}
		case '\\':
			if i+1 >= len(template) {
				return nil, fmt.Errorf("editor: %q: trailing backslash", template)
			}
			i++
			cur = append(cur, template[i])
			inWord = true
		case '"', '\'':
			quote := c
			j := i + 1
			for j < len(template) && template[j] != quote {
				j++
			}
			if j >= len(template) {
				return nil, fmt.Errorf("editor: %q: unterminated %c quote", template, quote)
			}
			cur = append(cur, template[i+1:j]...)
			inWord = true
			i = j
		default:
			cur = append(cur, c)
			inWord = true
		}
	}
	if inWord {
		argv = append(argv, string(cur))
	}
	if len(argv) == 0 {
		return nil, errors.New("editor: empty command")
	}
	return argv, nil
}
