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
	if tools := wf.MCP["fs"].Tools; tools == nil || !tools.All {
		t.Errorf("fs tools: got %+v, want all", tools)
	}
	if tools := wf.MCP["gh"].Tools; tools == nil || tools.All || len(tools.Names) != 2 {
		t.Errorf("gh tools: got %+v, want two names", tools)
	}
	if tools := wf.MCP["none"].Tools; tools == nil || tools.All || len(tools.Names) != 0 {
		t.Errorf("none tools: got %+v, want none", tools)
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
