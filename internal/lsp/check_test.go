package lsp

import (
	"errors"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		path string
		text string
		want store.DocumentKind
		err  error
	}{
		{name: "workflow", path: "wf.yaml", text: "openawf: 0.1.0\nname: x\n", want: store.Workflow},
		{name: "broken workflow", path: "wf.yaml", text: "openawf: 0.1.0\n  : [\n", want: store.Workflow},
		{name: "other yaml", path: "ci.yaml", text: "on: push\n", err: ErrNotOpenAWF},
		{name: "yml", path: "wf.yml", text: "openawf: 0.1.0\n", err: ErrNotOpenAWF},
		{name: "task", path: "t.md", text: "---\nopenawf: 0.1.0\n---\n\nbody\n", want: store.Task},
		{name: "unclosed task", path: "t.md", text: "---\nopenawf: 0.1.0\n", want: store.Task},
		{name: "no frontmatter", path: "t.md", text: "openawf: 0.1.0\n", err: ErrNotOpenAWF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detect(tt.path, []byte(tt.text))
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Errorf("got %v, %v, want %v, %v", got, err, tt.want, tt.err)
			}
		})
	}
}

func TestLineRange(t *testing.T) {
	lines := []string{"name: x  ", "  é: 😀v", ""}
	tests := []struct {
		name         string
		line, column int
		want         textRange
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

func span(line, start, end int) textRange {
	return textRange{Start: position{Line: line, Character: start}, End: position{Line: line, Character: end}}
}

func TestURI(t *testing.T) {
	tests := []struct {
		uri  string
		path string
	}{
		{uri: "file:///home/me/wf.yaml", path: "/home/me/wf.yaml"},
		{uri: "file:///home/me/a%20b%23c.yaml", path: "/home/me/a b#c.yaml"},
		{uri: "file:///C:/Users/me/wf.yaml", path: "C:/Users/me/wf.yaml"},
		{uri: "file://server/share/wf.yaml", path: "//server/share/wf.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			if got, err := slashPath(tt.uri); err != nil || got != tt.path {
				t.Errorf("slashPath: got %q, %v, want %q", got, err, tt.path)
			}
			if got := slashURI(tt.path); got != tt.uri {
				t.Errorf("slashURI: got %q, want %q", got, tt.uri)
			}
		})
	}
	if got, err := slashPath("file:///c%3A/Users/me/wf.yaml"); err != nil || got != "c:/Users/me/wf.yaml" {
		t.Errorf("encoded drive: got %q, %v", got, err)
	}
	if _, err := slashPath("untitled:Untitled-1"); !errors.Is(err, ErrNotFileURI) {
		t.Errorf("untitled: got %v, want %v", err, ErrNotFileURI)
	}
}
