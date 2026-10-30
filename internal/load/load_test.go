package load

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
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
	if len(w.Servers) != 1 || !w.Servers[0].Allow.All {
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
		{file: "noedge.yaml", want: []error{ErrNoEdge}},
		{file: "nooutcomes.yaml", want: []error{ErrNoOutcomes}},
		{file: "outcomeoutput.yaml", want: []error{schema.ErrOutcomesAndOutput}},
		{file: "outcomeproperty.yaml", want: []error{schema.ErrOutcomeProperty}},
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
	if names := f.Doc.Outcomes.Names(); names[0] != "ok" || names[1] != "fail" {
		t.Errorf("outcomes: got %q, want declaration order", names)
	}
}

func TestTaskOutcomeProperty(t *testing.T) {
	if _, err := ReadTask(filepath.Join("testdata", "outcomeproperty.md"), nil); !errors.Is(err, schema.ErrOutcomeProperty) {
		t.Errorf("got %v, want %v", err, schema.ErrOutcomeProperty)
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

func TestDuplicateModel(t *testing.T) {
	_, err := ReadWorkflow(filepath.Join("testdata", "dupmodel.yaml"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Errorf("got %v, want the duplicate model rejected", err)
	}
}

func TestMCPUses(t *testing.T) {
	_, err := ReadWorkflow(filepath.Join("testdata", "mcpuses.yaml"), nil, nil)
	var problems Problems
	if !errors.As(err, &problems) {
		t.Fatalf("got %v, want problems", err)
	}
	want := []struct {
		line, column int
		err          error
		text         string
	}{
		{10, 9, ErrNotDeclared, `"absent"`},
		{11, 9, ErrNotDeclared, `server "absent"`},
		{12, 9, ErrNoTool, `"gh/"`},
		{13, 9, ErrNotAllowed, `tool "delete_repo"`},
		{14, 9, ErrDenied, `tool "rm"`},
		{23, 22, manifest.ErrBadPlaceholder, `${1TOKEN}`},
		{28, 12, manifest.ErrBadPlaceholder, `${ROOT`},
		{30, 13, manifest.ErrBadPlaceholder, `${HOME-DIR}`},
	}
	if len(problems) != len(want) {
		t.Fatalf("got %d problems, want %d:\n%v", len(problems), len(want), err)
	}
	for i, w := range want {
		p := problems[i]
		if p.Line != w.line || p.Column != w.column || !errors.Is(p, w.err) || !strings.Contains(p.Message(), w.text) {
			t.Errorf("problem %d: got %v, want %d:%d %v naming %s", i, p, w.line, w.column, w.err, w.text)
		}
	}
}

func TestMCPServers(t *testing.T) {
	_, err := ReadWorkflow(filepath.Join("testdata", "mcpserver.yaml"), nil, nil)
	var problems Problems
	if !errors.As(err, &problems) {
		t.Fatalf("got %v, want problems", err)
	}
	want := []struct {
		path string
		err  error
	}{
		{"mcp/both/deny", schema.ErrAllowAndDeny},
		{"mcp/empty/allow", schema.ErrEmptyAllow},
		{"mcp/neither", schema.ErrNoCommand},
		{"mcp/twice/package", schema.ErrCommandAndPackage},
		{"mcp/remote", schema.ErrNoURL},
		{"mcp/remote/env", schema.ErrRemoteField},
		{"mcp/local/url", schema.ErrStdioField},
		{"mcp/latest/package/version", schema.ErrLatest},
		{"mcp/bundle/package", schema.ErrNoFileSHA},
	}
	for _, w := range want {
		found := false
		for _, p := range problems {
			if errors.Is(p, w.err) && strings.HasPrefix(p.Message(), w.path+":") {
				found = true
			}
		}
		if !found {
			t.Errorf("no %q at %s in:\n%v", w.err, w.path, err)
		}
	}
	if len(problems) != len(want) {
		t.Errorf("got %d problems, want %d:\n%v", len(problems), len(want), err)
	}
}

func TestPRReviewExample(t *testing.T) {
	f, err := ReadWorkflow(filepath.Join("testdata", "pr-review.yaml"), nil, nil)
	if err != nil {
		t.Fatalf("ReadWorkflow: %v", err)
	}
	var gather *awf.Task
	for _, s := range f.Compiled.States {
		if s.Task.Name == "gather" {
			gather = s.Task
		}
	}
	if gather == nil || len(gather.Uses) != 3 || gather.Uses[0].String() != "github/get_pull_request" || gather.Uses[2].Tool != "" {
		t.Fatalf("gather uses: got %+v", gather)
	}
	files := gather.Uses[2].Server
	if files.Package == nil || files.Package.RegistryType != "npm" || files.Args[0] != "${REPO_DIR}" || files.Permits("write_file") {
		t.Errorf("files: got %+v", files)
	}
}
