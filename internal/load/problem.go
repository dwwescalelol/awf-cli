package load

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/schema"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type Problem struct {
	File    string
	Line    int
	Column  int
	Warning bool
	Err     error
}

func (p *Problem) Error() string {
	severity := "error"
	if p.Warning {
		severity = "warning"
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", p.File, p.Line, p.Column, severity, p.Message())
}

func (p *Problem) Message() string {
	if y, ok := p.Err.(yaml.Error); ok {
		return y.GetMessage()
	}
	return p.Err.Error()
}

func (p *Problem) Unwrap() error { return p.Err }

type Problems []*Problem

func (ps Problems) Error() string {
	lines := make([]string, 0, len(ps))
	for _, p := range ps {
		lines = append(lines, p.Error())
	}
	return strings.Join(lines, "\n")
}

func (ps Problems) Unwrap() []error {
	out := make([]error, 0, len(ps))
	for _, p := range ps {
		out = append(out, p)
	}
	return out
}

type pathError struct {
	path  string
	value bool
	err   error
}

func (e *pathError) Error() string { return e.err.Error() }

func (e *pathError) Unwrap() error { return e.err }

func onKey(path string, err error) error {
	return &pathError{path: path, err: err}
}

func onValue(path string, err error) error {
	return &pathError{path: path, value: true, err: err}
}

type source struct {
	file string
	root ast.Node
}

func parse(path string, data []byte) (*source, error) {
	src := &source{file: display(path)}
	f, err := parser.ParseBytes(data, 0)
	if err != nil {
		var y yaml.Error
		if errors.As(err, &y) {
			err = y
		}
		return nil, Problems{src.problem(err, false)}
	}
	if len(f.Docs) > 0 {
		src.root = f.Docs[0].Body
	}
	return src, nil
}

func display(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if real, err := filepath.EvalSymlinks(wd); err == nil {
		wd = real
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	rel, err := filepath.Rel(wd, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func (src *source) report(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	problems := make(Problems, 0, len(errs))
	for _, err := range errs {
		problems = append(problems, src.problem(err, false))
	}
	slices.SortStableFunc(problems, func(a, b *Problem) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
	return problems
}

func (src *source) warnings(errs []error) Problems {
	out := make(Problems, 0, len(errs))
	for _, err := range errs {
		out = append(out, src.problem(err, true))
	}
	return out
}

func (src *source) problem(err error, warning bool) *Problem {
	if p, ok := err.(*Problem); ok {
		return p
	}
	p := &Problem{File: src.file, Line: 1, Column: 1, Warning: warning, Err: err}
	switch e := err.(type) {
	case *pathError:
		p.Line, p.Column = src.position(strings.Split(e.path, "/"), e.value)
	case *schema.Violation:
		p.Line, p.Column = src.position(e.Path, false)
	case *awf.StateError:
		p.Line, p.Column = src.position([]string{"orchestration", e.State}, false)
	case yaml.Error:
		if tk := e.GetToken(); tk != nil && tk.Position != nil {
			p.Line, p.Column = tk.Position.Line, tk.Position.Column
		}
	}
	return p
}

func (src *source) position(path []string, value bool) (int, int) {
	line, col := 1, 1
	node := src.root
	for _, segment := range path {
		key, next := child(node, segment)
		if key == nil {
			return line, col
		}
		line, col = place(key)
		node = next
	}
	if value && isScalar(node) {
		line, col = place(node)
	}
	return line, col
}

func child(node ast.Node, segment string) (ast.Node, ast.Node) {
	switch n := unwrap(node).(type) {
	case *ast.MappingNode:
		for _, entry := range n.Values {
			if keyText(entry.Key) == segment {
				return entry.Key, entry.Value
			}
		}
	case *ast.MappingValueNode:
		if keyText(n.Key) == segment {
			return n.Key, n.Value
		}
	case *ast.SequenceNode:
		i, err := strconv.Atoi(segment)
		if err == nil && i >= 0 && i < len(n.Values) {
			return n.Values[i], n.Values[i]
		}
	}
	return nil, nil
}

func unwrap(node ast.Node) ast.Node {
	for {
		switch n := node.(type) {
		case *ast.AnchorNode:
			node = n.Value
		case *ast.TagNode:
			node = n.Value
		default:
			return node
		}
	}
}

func keyText(key ast.MapKeyNode) string {
	if s, ok := unwrap(key).(*ast.StringNode); ok {
		return s.Value
	}
	return key.String()
}

func isScalar(node ast.Node) bool {
	switch unwrap(node).(type) {
	case nil, *ast.NullNode, *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		return false
	}
	return true
}

func place(node ast.Node) (int, int) {
	tk := node.GetToken()
	if tk == nil || tk.Position == nil {
		return 1, 1
	}
	return tk.Position.Line, tk.Position.Column
}
