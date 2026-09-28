package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestRenderStored(t *testing.T) {
	project(t)
	wf, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	task, err := create(store.Task, "build", "", false)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := renderPage(&out, store.Workflow, wf.path); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if !strings.Contains(out.String(), "<h1>deploy</h1>") || !strings.Contains(out.String(), `class="fsm"`) {
		t.Error("workflow page has no title or graph")
	}

	out.Reset()
	if err := renderPage(&out, store.Task, task.path); err != nil {
		t.Fatalf("task: %v", err)
	}
	if !strings.Contains(out.String(), "<h1>build</h1>") || strings.Contains(out.String(), `class="fsm"`) {
		t.Error("task page is not titled by its id, or draws a graph")
	}
}

func TestTaskName(t *testing.T) {
	tests := map[string]string{
		"/p/.awf/task/create-diff/0.1.0.md": "create-diff",
		"flows/ship.md":                     "ship",
	}
	for path, want := range tests {
		if got := taskName(path); got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
}
