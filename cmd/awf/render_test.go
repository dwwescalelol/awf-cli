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

	var out bytes.Buffer
	if err := renderPage(&out, wf.path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<h1>deploy</h1>") || !strings.Contains(out.String(), `class="fsm"`) {
		t.Error("page has no title or graph")
	}
}
