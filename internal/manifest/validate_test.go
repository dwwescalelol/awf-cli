package manifest

import (
	"os"
	"strings"
	"testing"
)

func TestValidateTestdata(t *testing.T) {
	wf := load(t, "testdata/feat-dev-small.yaml")
	if err := wf.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "edge to undefined task",
			yaml: header + "orchestration:\n  a: b\ntasks:\n  a:\n    body: hi\n",
			want: `invalid /orchestration/a: edge to undefined task "b"`,
		},
		{
			name: "start is not a task",
			yaml: "openawf: 0.1.0\nname: wf\nstart: z\norchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n",
			want: `invalid /start: "z" is not a defined task`,
		},
		{
			name: "task with no node",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n  b:\n    body: hi\n",
			want: "invalid /tasks/b: task has no orchestration node",
		},
		{
			name: "node with no task",
			yaml: header + "orchestration:\n  a: null\n  b: null\ntasks:\n  a:\n    body: hi\n",
			want: `invalid /orchestration/b: no task "b" is defined`,
		},
		{
			name: "outcome with no edge",
			yaml: header + "orchestration:\n  a:\n    pass: a\ntasks:\n  a:\n    outcomes: [pass, fail]\n    body: hi\n",
			want: `invalid /orchestration/a: task emits "fail", which has no edge`,
		},
		{
			name: "edge for an unemitted outcome",
			yaml: header + "orchestration:\n  a:\n    pass: a\n    nope: a\ntasks:\n  a:\n    outcomes: [pass]\n    body: hi\n",
			want: `invalid /orchestration/a/nope: "nope" is not an outcome of the task`,
		},
		{
			name: "outcomes but no branch",
			yaml: header + "orchestration:\n  a: a\ntasks:\n  a:\n    outcomes: [pass]\n    body: hi\n",
			want: "invalid /orchestration/a: task emits outcomes, so the node must branch",
		},
		{
			name: "branch but no outcomes",
			yaml: header + "orchestration:\n  a:\n    pass: a\ntasks:\n  a:\n    body: hi\n",
			want: "invalid /orchestration/a: node branches, but the task emits no outcomes",
		},
		{
			name: "undeclared server",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    uses: [git]\n    body: hi\n",
			want: `invalid /tasks/a/uses: "git" is not declared in mcp`,
		},
		{
			name: "version is not major.minor.patch",
			yaml: header + "version: 1.0\norchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n",
			want: `invalid /version: "1.0": not major.minor.patch`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			err = wf.Validate()
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func load(t *testing.T, path string) *Workflow {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return wf
}
