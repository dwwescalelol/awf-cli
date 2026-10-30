package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/goccy/go-yaml"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTaskRoundTrip(t *testing.T) {
	data := read(t, "task.md")
	task, err := UnmarshalTask(data)
	if err != nil {
		t.Fatalf("UnmarshalTask: %v", err)
	}
	if task.SHA != nil {
		t.Errorf("sha: got %q, want null", *task.SHA)
	}
	if names := task.Outcomes.Names(); len(names) != 2 || names[0] != "ok" || names[1] != "fail" {
		t.Errorf("outcomes: got %q, want ok then fail", names)
	}
	if task.System != "You write diffs." || task.Effort != Low || task.MaxTurns != 5 || task.Timeout != 30000 {
		t.Errorf("task fields: got %+v", task)
	}
	if task.License != "Apache-2.0" || task.ExternalVersion != "ext-3" || task.Metadata.Icon() != "./diff.svg" {
		t.Errorf("task identity: got %+v", task)
	}
	if len(task.Model) != 2 || task.Model[1] != "openai/gpt-6" {
		t.Errorf("model: got %q, want a fallback chain", task.Model)
	}
	if keys := extensionKeys(task.Extensions); !slices.Equal(keys, []string{"x-runner", "x-claude"}) {
		t.Errorf("extensions: got %q, want x-runner then x-claude", keys)
	}
	if want := "# create-diff\n\nDescribe the work this task performs, and what it returns.\n"; string(task.Body) != want {
		t.Errorf("body: got %q, want %q", task.Body, want)
	}
	got, err := MarshalTask(task)
	if err != nil {
		t.Fatalf("MarshalTask: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("round trip:\ngot:\n%s\nwant:\n%s", got, data)
	}
}

func TestWorkflowRoundTrip(t *testing.T) {
	data := read(t, "workflow.yaml")
	wf, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if e := wf.Orchestration["plan"].Edge; e == nil || e.Task != "review" {
		t.Errorf("plan: got %+v, want edge to review", wf.Orchestration["plan"])
	}
	if b := wf.Orchestration["review"].Branch; b["fail"].Task != "plan" || b["fail"].Retries == nil || *b["fail"].Retries != 2 || b["ok"].Task != "ship" {
		t.Errorf("review: got %+v, want branch ok->ship, fail->plan x2", b)
	}
	if tr := wf.Orchestration["ship"]; tr.Edge != nil || tr.Branch != nil {
		t.Errorf("ship: got %+v, want terminal", tr)
	}
	if ref := wf.Tasks["ship"].Ref; ref != "./ship.md" {
		t.Errorf("ship ref: got %q, want ./ship.md", ref)
	}
	if task := wf.Tasks["plan"].Task; task == nil || task.Body != "# plan\n" {
		t.Errorf("plan task: got %+v", task)
	}
	review := wf.Tasks["review"].Task
	if names := review.Outcomes.Names(); len(names) != 2 || names[0] != "ok" || names[1] != "fail" {
		t.Errorf("review outcomes: got %q, want ok then fail", names)
	}
	if fail := review.Outcomes[1].Schema; fail["type"] != "object" || fail["properties"] == nil {
		t.Errorf("fail schema: got %+v", fail)
	}
	if len(review.Model) != 2 || review.Model[0] != "anthropic/claude-sonnet-5" {
		t.Errorf("review model: got %q, want a fallback chain", review.Model)
	}
	if review.Effort != High || review.MaxTurns != 10 || review.Timeout != 600000 || review.License != "MIT" || review.ExternalVersion != "rev-7" {
		t.Errorf("review fields: got %+v", review)
	}
	if want := "You review changes.\nYou never edit files.\n"; string(review.System) != want {
		t.Errorf("review system: got %q, want %q", review.System, want)
	}
	if tags := review.Metadata.Tags(); !slices.Equal(tags, []string{"review"}) {
		t.Errorf("review tags: got %q", tags)
	}
	if keys := extensionKeys(review.Extensions); !slices.Equal(keys, []string{"x-zed", "x-alpha"}) {
		t.Errorf("review extensions: got %q, want x-zed then x-alpha", keys)
	}
	if value, ok := review.Extensions.Value("x-alpha"); !ok || value != nil {
		t.Errorf("x-alpha: got %v %v, want a kept null", value, ok)
	}
	fail := wf.Orchestration["review"].Branch["fail"]
	if fail.Session != Resume {
		t.Errorf("fail edge session: got %q, want resume", fail.Session)
	}
	if _, ok := fail.Extensions.Value("x-runner"); !ok {
		t.Errorf("fail edge: got %+v, want x-runner kept", fail)
	}
	if fs := wf.MCP["fs"]; fs.Type != "stdio" || fs.Allow == nil || !fs.Allow.All || fs.Rest["command"] != "mcp-fs" {
		t.Errorf("fs: got %+v, want stdio allowing all tools", fs)
	}
	if fs := wf.MCP["fs"]; fs.Rest["x-runner"] != nil || len(fs.Extensions) != 1 {
		t.Errorf("fs extensions: got %+v, want x-runner as an extension only", fs)
	}
	if allow := wf.MCP["gh"].Allow; allow == nil || allow.All || len(allow.Names) != 2 {
		t.Errorf("gh allow: got %+v, want two names", allow)
	}
	if none := wf.MCP["none"]; none.Allow != nil || len(none.Deny) != 1 {
		t.Errorf("none: got %+v, want one denied tool", none)
	}
	if wf.License != "Apache-2.0" || wf.ExternalVersion != "n8n-41" || wf.Effort != Medium {
		t.Errorf("workflow fields: got %+v", wf)
	}
	if !slices.Equal(wf.Metadata.Tags(), []string{"dev"}) || !slices.Equal(wf.Metadata.Authors(), []string{"Ada"}) || wf.Metadata.Icon() != "./icon.svg" {
		t.Errorf("metadata: got %+v", wf.Metadata)
	}
	if value, ok := wf.Metadata.Value("zeta"); !ok || value != nil {
		t.Errorf("metadata zeta: got %v %v, want a kept null", value, ok)
	}
	if keys := extensionKeys(wf.Extensions); !slices.Equal(keys, []string{"x-runner", "x-claude"}) {
		t.Errorf("extensions: got %q, want x-runner then x-claude", keys)
	}

	got, err := Marshal(wf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("round trip:\ngot:\n%s\nwant:\n%s", got, data)
	}
}

func extensionKeys(e Extensions) []string {
	keys := make([]string, len(e))
	for i, item := range e {
		keys[i], _ = item.Key.(string)
	}
	return keys
}

func TestModels(t *testing.T) {
	tests := []struct {
		text string
		want error
	}{
		{text: "model: anthropic/claude-opus-5\n"},
		{text: "model:\n  - anthropic/claude-opus-5\n  - openai/gpt-6\n"},
		{text: "model:\n  - anthropic/claude-opus-5\n  - anthropic/claude-opus-5\n", want: ErrDuplicateModel},
		{text: "model: []\n", want: ErrNoModels},
		{text: "model: 3\n", want: ErrNotModel},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			var task Task
			err := yaml.Unmarshal([]byte(tt.text), &task)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMarshalIndentedBody(t *testing.T) {
	wf := &Workflow{Name: "x", Start: "a", Orchestration: Orchestration{"a": Transition{}}, Tasks: Tasks{"a": TaskEntry{Task: &Task{Body: "  indented\nnext\n"}}}}
	data, err := Marshal(wf)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if body := got.Tasks["a"].Task.Body; body != wf.Tasks["a"].Task.Body {
		t.Errorf("got %q, want %q", body, wf.Tasks["a"].Task.Body)
	}
}

func TestDeclares(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "workflow", text: "openawf: 0.1.0\nname: x\n", want: true},
		{name: "later key", text: "name: x\n\"openawf\": 0.1.0\n", want: true},
		{name: "frontmatter", text: "---\nopenawf: 0.1.0\n---\n\nbody\n", want: true},
		{name: "nested", text: "meta:\n  openawf: 0.1.0\n"},
		{name: "block scalar", text: "body: |\n  openawf: 0.1.0\n"},
		{name: "later document", text: "name: x\n---\nopenawf: 0.1.0\n"},
		{name: "task body", text: "---\ntitle: x\n---\nopenawf: 0.1.0\n"},
		{name: "plain scalar", text: "openawf:x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Declares([]byte(tt.text)); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
