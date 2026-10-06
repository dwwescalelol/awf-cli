package lsp

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
)

func TestLineRange(t *testing.T) {
	lines := []string{"name: x  ", "  é: 😀v", ""}
	tests := []struct {
		name         string
		line, column int
		want         Range
	}{
		{name: "first line", line: 1, column: 7, want: span(0, 6, 7)},
		{name: "utf16 columns", line: 2, column: 7, want: span(1, 7, 8)},
		{name: "empty line", line: 3, column: 1, want: span(2, 0, 0)},
		{name: "past the end", line: 9, column: 4, want: span(2, 0, 0)},
		{name: "past the column", line: 1, column: 40, want: span(0, 9, 9)},
		{name: "zero", line: 0, column: 0, want: span(0, 0, 7)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lineRange(lines, tt.line, tt.column); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func span(line, start, end int) Range {
	return Range{Start: Position{Line: line, Character: start}, End: Position{Line: line, Character: end}}
}

func TestDiagnostic(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "wf.yaml")
	task := filepath.Join(dir, "task.md")
	lines := []string{"openawf: 0.1.0", "tasks:", "  a:", "    $ref: task.md"}
	sites := []site{{rng: span(3, 10, 17), target: canonical(task)}}
	cause := errors.New("bad")
	tests := []struct {
		name     string
		problem  load.Problem
		want     Range
		severity int
		message  string
	}{
		{name: "here", problem: load.Problem{File: here, Line: 2, Column: 1, Err: cause}, want: span(1, 0, 6), severity: severityError, message: "bad"},
		{name: "warning", problem: load.Problem{File: here, Line: 1, Column: 10, Warning: true, Err: cause}, want: span(0, 9, 14), severity: severityWarning, message: "bad"},
		{name: "ref target", problem: load.Problem{File: task, Line: 5, Column: 2, Err: cause}, want: span(3, 10, 17), severity: severityError, message: task + ":5:2: bad"},
		{name: "unknown file", problem: load.Problem{File: filepath.Join(dir, "x.md"), Line: 5, Column: 2, Err: cause}, want: Range{}, severity: severityError, message: filepath.Join(dir, "x.md") + ":5:2: bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnostic(&tt.problem, lines, canonical(here), sites)
			if got.Range != tt.want || got.Severity != tt.severity || got.Message != tt.message {
				t.Errorf("got %+v, want %+v severity %d %q", got, tt.want, tt.severity, tt.message)
			}
		})
	}
}
