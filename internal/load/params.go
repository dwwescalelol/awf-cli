package load

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/prompt"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"
)

var (
	ErrUnknownField = errors.New("names no field in the top-level properties of the task's input")
	ErrNoInput      = errors.New("the task has no input")
)

// textError is an error at a byte offset inside the string at path.
type textError struct {
	path   string
	text   string
	offset int
	err    error
}

func (e *textError) Error() string { return e.err.Error() }

func (e *textError) Unwrap() error { return e.err }

// lineError is an error at a line and column of the file.
type lineError struct {
	line, column int
	err          error
}

func (e *lineError) Error() string { return e.err.Error() }

func (e *lineError) Unwrap() error { return e.err }

// locate places an error found at offset in the text of a task's key.
type locate func(key, text string, offset int, err error) error

// inYAML places an error inside the scalar at prefix + key.
func inYAML(prefix string) locate {
	return func(key, text string, offset int, err error) error {
		return &textError{path: prefix + key, text: text, offset: offset, err: err}
	}
}

// params reports each prompt parameter in the task's system and body that
// names no field of its input (spec section 7.1).
func params(prefix string, t *manifest.Task, at locate) []error {
	fields := inputFields(t.Input)
	var errs []error
	for _, key := range []string{"system", "body"} {
		text := string(t.System)
		if key == "body" {
			text = string(t.Body)
		}
		for _, p := range prompt.Params(text) {
			cause := ErrNoInput
			switch {
			case t.Input == nil:
			case slices.Contains(fields, p.Field):
				continue
			default:
				cause = ErrUnknownField
			}
			err := fmt.Errorf("%s%s: prompt parameter %s: %w", prefix, key, text[p.Offset:p.End], cause)
			errs = append(errs, at(key, text, p.Offset, err))
		}
	}
	return errs
}

func inputFields(input map[string]any) []string {
	props, _ := input["properties"].(map[string]any)
	out := make([]string, 0, len(props))
	for name := range props {
		out = append(out, name)
	}
	return out
}

// within places the byte offset of text, the string at path. It is exact for
// a literal block scalar and for a scalar on one line whose source reads as
// its value. Elsewhere, such as in a folded block scalar or a quoted scalar
// with escapes, it falls back to the start of the scalar.
func (src *source) within(path []string, text string, offset int) (int, int) {
	line, col := src.position(path, true)
	head := text[:offset]
	row := strings.Count(head, "\n")
	run := utf8.RuneCountInString(head[strings.LastIndexByte(head, '\n')+1:])
	value := strings.Split(text, "\n")[row]

	switch n := unwrap(src.value(path)).(type) {
	case *ast.LiteralNode:
		if n.Start == nil || n.Start.Type != token.LiteralType || n.Start.Position == nil {
			break
		}
		source, ok := src.line(n.Start.Position.Line + 1 + row)
		if !ok || !strings.HasSuffix(source, value) {
			break
		}
		indent := utf8.RuneCountInString(source) - utf8.RuneCountInString(value)
		return n.Start.Position.Line + 1 + row, indent + run + 1
	case *ast.StringNode:
		if strings.Contains(text, "\n") {
			break
		}
		source, ok := src.line(line)
		if !ok {
			break
		}
		runes := []rune(source)
		for _, skip := range []int{0, 1} {
			at := col - 1 + skip
			if at <= len(runes) && strings.HasPrefix(string(runes[at:]), text) {
				return line, col + skip + run
			}
		}
	}
	return line, col
}

// value finds the node at path, or nil.
func (src *source) value(path []string) ast.Node {
	node := src.root
	for _, segment := range path {
		key, next := child(node, segment)
		if key == nil {
			return nil
		}
		node = next
	}
	return node
}

// line returns the text of a 1-based line of the source.
func (src *source) line(n int) (string, bool) {
	if n < 1 || n > len(src.lines) {
		return "", false
	}
	return src.lines[n-1], true
}

// inMarkdown places an error in a task file whose body starts at line first,
// and in its frontmatter otherwise.
func inMarkdown(first int) locate {
	frontmatter := inYAML("")
	return func(key, text string, offset int, err error) error {
		if key != "body" {
			return frontmatter(key, text, offset, err)
		}
		head := text[:offset]
		line := first + strings.Count(head, "\n")
		column := utf8.RuneCountInString(head[strings.LastIndexByte(head, '\n')+1:]) + 1
		return &lineError{line: line, column: column, err: err}
	}
}
