package lsp

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/dwwescalelol/awf-cli/internal/load"
)

const source = "awf"

func diagnose(path string, text []byte, problems []*load.Problem, sites []site) []Diagnostic {
	lines := splitLines(text)
	here := canonical(path)
	out := make([]Diagnostic, 0, len(problems))
	for _, p := range problems {
		out = append(out, diagnostic(p, lines, here, sites))
	}
	slices.SortStableFunc(out, func(a, b Diagnostic) int {
		return cmp.Or(cmp.Compare(a.Range.Start.Line, b.Range.Start.Line), cmp.Compare(a.Range.Start.Character, b.Range.Start.Character))
	})
	return out
}

func diagnostic(p *load.Problem, lines []string, here string, sites []site) Diagnostic {
	d := Diagnostic{Severity: severityError, Source: source, Message: p.Message()}
	if p.Warning {
		d.Severity = severityWarning
	}
	if canonical(p.File) == here {
		d.Range = lineRange(lines, p.Line, p.Column)
		return d
	}
	d.Message = fmt.Sprintf("%s:%d:%d: %s", p.File, p.Line, p.Column, p.Message())
	target := canonical(p.File)
	for _, s := range sites {
		if s.target == target {
			d.Range = s.rng
			break
		}
	}
	return d
}

func problemsOf(path string, err error, warnings []error) []*load.Problem {
	var out []*load.Problem
	var ps load.Problems
	switch {
	case errors.As(err, &ps):
		out = append(out, ps...)
	case err != nil:
		out = append(out, &load.Problem{File: path, Line: 1, Column: 1, Err: err})
	}
	for _, w := range warnings {
		var p *load.Problem
		if errors.As(w, &p) {
			out = append(out, p)
		}
	}
	return out
}

func splitLines(text []byte) []string {
	return strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
}

func lineRange(lines []string, line, column int) Range {
	l := min(max(line-1, 0), len(lines)-1)
	runes := []rune(lines[l])
	start := min(max(column-1, 0), len(runes))
	end := len([]rune(strings.TrimRight(string(runes), " \t")))
	start16 := len(utf16.Encode(runes[:start]))
	end16 := max(len(utf16.Encode(runes[:max(end, start)])), start16)
	return Range{Start: Position{Line: l, Character: start16}, End: Position{Line: l, Character: end16}}
}
