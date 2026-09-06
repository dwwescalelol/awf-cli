package manifest

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

const header = "openawf: 0.1.0\nname: wf\nstart: a\n"

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want Workflow
	}{
		{
			name: "terminal transition",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {}},
				Tasks:         Tasks{"a": {Task: &Task{Body: "hi"}}},
			},
		},
		{
			name: "string edge",
			yaml: header + "orchestration:\n  a: b\ntasks:\n  a:\n    body: hi\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {Edge: &Edge{Task: "b"}}},
				Tasks:         Tasks{"a": {Task: &Task{Body: "hi"}}},
			},
		},
		{
			name: "edge object",
			yaml: header + "orchestration:\n  a:\n    task: b\n    retries: 2\n    session: resume\ntasks:\n  a:\n    body: hi\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {Edge: &Edge{Task: "b", Retries: ptr(2), Session: "resume"}}},
				Tasks:         Tasks{"a": {Task: &Task{Body: "hi"}}},
			},
		},
		{
			name: "branch",
			yaml: header + "orchestration:\n  a:\n    pass: b\n    fail:\n      task: a\n      retries: 0\ntasks:\n  a:\n    body: hi\n",
			want: Workflow{
				OpenAWF: "0.1.0",
				Name:    "wf",
				Start:   "a",
				Orchestration: Orchestration{"a": {Branch: Branch{
					"pass": {Task: "b"},
					"fail": {Task: "a", Retries: ptr(0)},
				}}},
				Tasks: Tasks{"a": {Task: &Task{Body: "hi"}}},
			},
		},
		{
			name: "task ref",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    $ref: ./a.md\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {}},
				Tasks:         Tasks{"a": {Ref: "./a.md"}},
			},
		},
		{
			name: "mcp tools wildcard",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\nmcp:\n  git:\n    transport: stdio\n    tools: \"*\"\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {}},
				Tasks:         Tasks{"a": {Task: &Task{Body: "hi"}}},
				MCP:           map[string]MCPServer{"git": {Transport: "stdio", Tools: &MCPTools{All: true}}},
			},
		},
		{
			name: "mcp tools list",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\nmcp:\n  git:\n    tools: [commit, diff]\n",
			want: Workflow{
				OpenAWF:       "0.1.0",
				Name:          "wf",
				Start:         "a",
				Orchestration: Orchestration{"a": {}},
				Tasks:         Tasks{"a": {Task: &Task{Body: "hi"}}},
				MCP:           map[string]MCPServer{"git": {Tools: &MCPTools{Names: []string{"commit", "diff"}}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("got %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		yaml string
		want Version
	}{
		{"1.0", "1.0"},
		{"1", "1"},
		{`"1.0"`, "1.0"},
		{"0.1.0", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.yaml, func(t *testing.T) {
			src := header + "version: " + tt.yaml + "\norchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n"
			wf, err := Parse([]byte(src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if wf.Version != tt.want {
				t.Fatalf("got %q, want %q", wf.Version, tt.want)
			}

			out, err := Marshal(wf)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			again, err := Parse(out)
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again.Version != tt.want {
				t.Fatalf("reparsed %q, want %q", again.Version, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "null edge in branch",
			yaml: header + "orchestration:\n  testing:\n    pass: a\n    fail: null\ntasks:\n  a:\n    body: hi\n",
			want: "parse /orchestration/testing/fail: edge null: not a task name or an edge",
		},
		{
			name: "unknown edge field",
			yaml: header + "orchestration:\n  a:\n    task: b\n    pass: c\ntasks:\n  a:\n    body: hi\n",
			want: "parse /orchestration/a: ",
		},
		{
			name: "unknown task field",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n    colour: red\n",
			want: "parse /tasks/a: ",
		},
		{
			name: "bad tools",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\nmcp:\n  git:\n    tools: some\n",
			want: `parse: tools "some": not a tool list or "*"`,
		},
		{
			name: "header field",
			yaml: "openawf: 0.1.0\nname: [1]\nstart: a\norchestration:\n  a: null\ntasks:\n  a:\n    body: hi\n",
			want: "parse: ",
		},
		{
			name: "unknown top level field",
			yaml: header + "orchestration:\n  a: null\ntasks:\n  a:\n    body: hi\nnope: 1\n",
			want: "parse: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("want *ParseError, got %T", err)
			}
			if pe.Unwrap() == nil {
				t.Error("Unwrap returned nil")
			}
			if !strings.HasPrefix(err.Error(), tt.want) {
				t.Fatalf("got %q, want prefix %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	src, err := os.ReadFile("testdata/feat-dev-small.yaml")
	if err != nil {
		t.Fatal(err)
	}
	wf, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Marshal(wf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	again, err := Parse(out)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(wf, again) {
		t.Errorf("round trip differs\n%s", out)
	}
	if !wf.Orchestration["remove-worktree"].Terminal() {
		t.Error("remove-worktree should be terminal")
	}
	if wf.Tasks["testing"].Task.Version != "2.1.0" {
		t.Errorf("task version %q", wf.Tasks["testing"].Task.Version)
	}
}
