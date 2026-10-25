package manifest

import (
	"os"
	"path/filepath"
	"testing"
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
	if task.Rest["system"] != "You write diffs." {
		t.Errorf("rest: got %+v, want system kept", task.Rest)
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
	if _, ok := review.Rest["body"]; ok || review.Rest["effort"] != "high" {
		t.Errorf("review rest: got %+v, want effort and max-turns only", review.Rest)
	}
	if e := wf.Orchestration["review"].Branch["fail"]; e.Rest["x-runner"] == nil {
		t.Errorf("fail edge: got %+v, want x-runner kept", e)
	}
	if fs := wf.MCP["fs"]; fs.Type != "stdio" || fs.Allow == nil || !fs.Allow.All || fs.Rest["command"] != "mcp-fs" {
		t.Errorf("fs: got %+v, want stdio allowing all tools", fs)
	}
	if allow := wf.MCP["gh"].Allow; allow == nil || allow.All || len(allow.Names) != 2 {
		t.Errorf("gh allow: got %+v, want two names", allow)
	}
	if none := wf.MCP["none"]; none.Allow != nil || len(none.Deny) != 1 {
		t.Errorf("none: got %+v, want one denied tool", none)
	}
	if wf.Rest["license"] != "Apache-2.0" || wf.Rest["x-runner"] == nil {
		t.Errorf("rest: got %+v, want license and x-runner kept", wf.Rest)
	}

	got, err := Marshal(wf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("round trip:\ngot:\n%s\nwant:\n%s", got, data)
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
