package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/page"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestRenderPage(t *testing.T) {
	project(t)
	wf, err := create(store.Workflow, "deploy", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := create(store.Task, "build", false); err != nil {
		t.Fatal(err)
	}

	data, err := renderPage([]string{"deploy"}, "", false, false)
	if err != nil || !strings.Contains(string(data), "<h1>deploy</h1>") {
		t.Fatalf("workflow: got %v", err)
	}
	data, err = renderPage([]string{"build"}, "", true, false)
	if err != nil || !strings.Contains(string(data), "<h1>build</h1>") {
		t.Fatalf("task: got %v", err)
	}

	rewrite(t, wf.path, "start: start", "start: nowhere")
	data, err = renderPage(nil, wf.path, false, false)
	if !errors.Is(err, page.ErrInvalid) || !strings.Contains(string(data), `class="overlay"`) {
		t.Errorf("invalid: got %v, want %v and the page", err, page.ErrInvalid)
	}
}
