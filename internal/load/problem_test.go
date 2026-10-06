package load

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

type located struct {
	file      string
	line, col int
	want      error
}

func problems(t *testing.T, err error) Problems {
	t.Helper()
	var out Problems
	if !errors.As(err, &out) {
		t.Fatalf("got %T %v, want Problems", err, err)
	}
	return out
}

func check(t *testing.T, got []*Problem, want []located) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d problems %v, want %d", len(got), got, len(want))
	}
	for i, w := range want {
		p := got[i]
		if filepath.Base(p.File) != w.file || p.Line != w.line || p.Column != w.col {
			t.Errorf("got %s:%d:%d, want %s:%d:%d", p.File, p.Line, p.Column, w.file, w.line, w.col)
		}
		if w.want != nil && !errors.Is(p, w.want) {
			t.Errorf("got %v, want %v", p, w.want)
		}
	}
}

func TestPositions(t *testing.T) {
	tests := []struct {
		file string
		want []located
	}{
		{file: "syntax.yaml", want: []located{{file: "syntax.yaml", line: 3, col: 8}}},
		{file: "extra.yaml", want: []located{{file: "extra.yaml", line: 9, col: 5}}},
		{file: "unbound.yaml", want: []located{
			{file: "unbound.yaml", line: 6, col: 9, want: ErrNotATask},
			{file: "unbound.yaml", line: 10, col: 9, want: ErrNotDeclared},
		}},
		{file: "trapped.yaml", want: []located{{file: "trapped.yaml", line: 5, col: 1, want: awf.ErrNoTerminal}}},
		{file: "looping.yaml", want: []located{
			{file: "looping.yaml", line: 9, col: 3, want: awf.ErrNoPath},
			{file: "looping.yaml", line: 10, col: 3, want: awf.ErrNoPath},
		}},
		{file: "taskref.yaml", want: []located{{file: "badtask.md", line: 3, col: 1, want: ErrUnresolvedRef}}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			_, err := ReadWorkflow(filepath.Join("testdata", tt.file), nil, nil)
			check(t, problems(t, err), tt.want)
		})
	}
}

func TestWarningPositions(t *testing.T) {
	f, err := ReadWorkflow(filepath.Join("testdata", "unreachable.yaml"), nil, nil)
	if err != nil {
		t.Fatalf("ReadWorkflow: %v", err)
	}
	var got []*Problem
	for _, w := range f.Warnings {
		got = append(got, w.(*Problem))
	}
	check(t, got, []located{{file: "unreachable.yaml", line: 7, col: 3, want: awf.ErrUnreachable}})
	if want := fmt.Sprintf("%s:7:3: warning: orchestration/cleanup: unreachable from start", got[0].File); got[0].Error() != want {
		t.Errorf("got %q, want %q", got[0].Error(), want)
	}
}

func TestTaskPositions(t *testing.T) {
	_, err := ReadTask(filepath.Join("testdata", "badtask.md"), nil)
	check(t, problems(t, err), []located{{file: "badtask.md", line: 3, col: 1}})

	_, err = ReadTask(filepath.Join("testdata", "bodykey.md"), nil)
	check(t, problems(t, err), []located{{file: "bodykey.md", line: 3, col: 1, want: manifest.ErrReservedBody}})
}
