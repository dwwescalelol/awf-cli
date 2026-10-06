package load

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

func TestWorkflow(t *testing.T) {
	f, err := ReadWorkflow(filepath.Join("testdata", "valid.yaml"), nil, nil)
	if err != nil {
		t.Fatalf("ReadWorkflow: %v", err)
	}
	w := f.Compiled
	if w.Start == nil || w.Start.Task.Name != "plan" {
		t.Errorf("start: got %+v, want plan", w.Start)
	}
	if len(w.States) != 3 {
		t.Errorf("states: got %d, want 3", len(w.States))
	}
	if len(w.Servers) != 1 || !w.Servers[0].Tools.All {
		t.Errorf("servers: got %+v, want gh with all tools", w.Servers)
	}
}

func TestWorkflowErrors(t *testing.T) {
	tests := []struct {
		file string
		want []error
	}{
		{file: "unbound.yaml", want: []error{ErrNotATask, ErrNotDeclared}},
		{file: "trapped.yaml", want: []error{awf.ErrNoTerminal}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			_, err := ReadWorkflow(filepath.Join("testdata", tt.file), nil, nil)
			for _, want := range tt.want {
				if !errors.Is(err, want) {
					t.Errorf("got %v, want %v", err, want)
				}
			}
		})
	}
}

func TestWorkflowSchema(t *testing.T) {
	if _, err := ReadWorkflow(filepath.Join("testdata", "schema.yaml"), nil, nil); err == nil {
		t.Fatal("missing start: got no error")
	}
}

func TestWorkflowMissingFile(t *testing.T) {
	if _, err := ReadWorkflow(filepath.Join("testdata", "absent.yaml"), nil, nil); err == nil {
		t.Fatal("got no error")
	}
}

func TestTask(t *testing.T) {
	f, err := ReadTask(filepath.Join("testdata", "task.md"), nil)
	if err != nil {
		t.Fatalf("ReadTask: %v", err)
	}
	if f.Doc.OpenAWF.String() != "0.1.0" || len(f.Doc.Outcomes) != 2 {
		t.Errorf("got %+v", f.Doc)
	}
}

func TestTaskErrors(t *testing.T) {
	for _, file := range []string{"badtask.md", "noversion.md", "valid.yaml", "bodykey.md"} {
		t.Run(file, func(t *testing.T) {
			if _, err := ReadTask(filepath.Join("testdata", file), nil); err == nil {
				t.Error("got no error")
			}
		})
	}
}

func TestTaskReservedBody(t *testing.T) {
	if _, err := ReadTask(filepath.Join("testdata", "bodykey.md"), nil); !errors.Is(err, manifest.ErrReservedBody) {
		t.Errorf("got %v, want %v", err, manifest.ErrReservedBody)
	}
}
