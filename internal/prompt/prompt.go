// Package prompt reads the prompt parameters of spec section 4.3 in a task's
// body or system.
package prompt

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Part is one piece of a prompt. A literal part has Text, with escapes
// resolved. A parameter part has Field, the name between its braces.
type Part struct {
	// Text is the literal text of a literal part.
	Text string
	// Field is the field a parameter part names. It is empty for a literal
	// part.
	Field string
	// Offset is the byte offset of the part's first byte in the prompt.
	Offset int
	// End is the byte offset just past the part's last byte in the prompt.
	End int
}

// IsParam reports whether the part is a prompt parameter.
func (p Part) IsParam() bool { return p.Field != "" }

// Parse splits text into literal parts and prompt parameters in one left to
// right pass. Code spans and fenced code are read like any other text.
//
// A prompt parameter is {{, zero or more U+0020 spaces, a name, zero or more
// spaces, then }}. A name is one or more characters that are neither white
// space nor a brace. Parameters do not nest: in {{{a}}} the outer braces are
// literal. \{{ is a literal {{. \\{{ is a literal \ followed by {{, which
// starts a parameter. Every other backslash is literal.
func Parse(text string) []Part {
	var parts []Part
	var lit strings.Builder
	start := 0
	flush := func(end int) {
		if end > start {
			parts = append(parts, Part{Text: lit.String(), Offset: start, End: end})
		}
		lit.Reset()
	}
	for i := 0; i < len(text); {
		rest := text[i:]
		switch {
		case strings.HasPrefix(rest, `\\{{`):
			lit.WriteByte('\\')
			i += 2
		case strings.HasPrefix(rest, `\{{`):
			lit.WriteString("{{")
			i += 3
		case strings.HasPrefix(rest, "{{"):
			field, n := param(rest)
			if n == 0 {
				lit.WriteByte('{')
				i++
				continue
			}
			flush(i)
			parts = append(parts, Part{Field: field, Offset: i, End: i + n})
			i += n
			start = i
		default:
			_, size := utf8.DecodeRuneInString(rest)
			lit.WriteString(rest[:size])
			i += size
		}
	}
	flush(len(text))
	return parts
}

// Params lists the prompt parameters in text.
func Params(text string) []Part {
	var out []Part
	for _, p := range Parse(text) {
		if p.IsParam() {
			out = append(out, p)
		}
	}
	return out
}

// param reads a prompt parameter at the start of s, which begins with {{. It
// returns the field and the parameter's length in bytes, or a length of 0
// when s does not start with a parameter.
func param(s string) (string, int) {
	i := 2
	for i < len(s) && s[i] == ' ' {
		i++
	}
	name := i
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '{' || r == '}' || unicode.IsSpace(r) {
			break
		}
		i += size
	}
	if i == name {
		return "", 0
	}
	field := s[name:i]
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if !strings.HasPrefix(s[i:], "}}") {
		return "", 0
	}
	return field, i + 2
}
