package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/render"
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
	if err := renderWorkflow(&out, wf.path, nil, render.Scope{}, ""); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	page := out.String()
	if !strings.Contains(page, "<h1>deploy</h1>") || !strings.Contains(page, `class="fsm"`) {
		t.Error("workflow page has no title or graph")
	}
	if strings.Contains(page, `class="bar"`) || strings.Contains(page, "EventSource") {
		t.Error("static workflow page has navigation or live reload")
	}

	out.Reset()
	if err := renderTask(&out, task.path, nil); err != nil {
		t.Fatalf("task: %v", err)
	}
	page = out.String()
	if !strings.Contains(page, "<h1>build</h1>") || strings.Contains(page, `class="fsm"`) {
		t.Error("task page is not titled by its id, or draws a graph")
	}
}

func TestRenderInvalid(t *testing.T) {
	project(t)
	good, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	bad, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, bad.path, "start: start", "start: nowhere")

	var out bytes.Buffer
	if err := renderWorkflow(&out, bad.path, nil, render.Scope{}, ""); !errors.Is(err, errInvalid) {
		t.Fatalf("got %v, want %v", err, errInvalid)
	}
	page := out.String()
	if !strings.Contains(page, "<h1>deploy</h1>") || !strings.Contains(page, `class="overlay"`) || !strings.Contains(page, "nowhere") {
		t.Error("invalid page has no header, overlay or validate error")
	}

	s, err := store.Resolve(false)
	if err != nil {
		t.Fatal(err)
	}
	scope, _, err := links(s, stored{id: "deploy", version: good.version})
	if err != nil {
		t.Fatalf("a malformed sibling failed the listing: %v", err)
	}
	if len(scope.Versions) != 2 || !scope.Versions[0].Invalid || scope.Versions[1].Invalid {
		t.Errorf("versions: got %+v, want the newest flagged invalid", scope.Versions)
	}
}
