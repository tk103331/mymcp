package app

import (
	"fmt"
	"strings"
	"unicode"
)

// splitArgs parses a command's arguments, allowing single and double quotes
// so paths and JSON values with spaces survive the round trip. Backslash
// escapes the next character outside single quotes.
func splitArgs(text string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	inToken := false
	flush := func() {
		if inToken {
			args = append(args, current.String())
			current.Reset()
			inToken = false
		}
	}
	for _, r := range text {
		if escaped {
			switch r {
			case 'n':
				current.WriteByte('\n')
			case 't':
				current.WriteByte('\t')
			case 'r':
				current.WriteByte('\r')
			default:
				current.WriteRune(r)
			}
			escaped = false
			inToken = true
			continue
		}
		switch {
		case r == '\\' && quote != '\'':
			escaped = true
			inToken = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			inToken = true
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case unicode.IsSpace(r):
			flush()
		default:
			current.WriteRune(r)
			inToken = true
		}
	}
	if escaped {
		current.WriteByte('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	flush()
	return args, nil
}

// joinArgs formats arguments for the single-line editor in splitArgs's
// syntax.
func joinArgs(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'\\") {
			arg = strings.ReplaceAll(arg, `\`, `\\`)
			arg = strings.ReplaceAll(arg, `"`, `\"`)
			arg = strings.ReplaceAll(arg, "\n", `\n`)
			arg = strings.ReplaceAll(arg, "\t", `\t`)
			arg = strings.ReplaceAll(arg, "\r", `\r`)
			parts[i] = `"` + arg + `"`
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
