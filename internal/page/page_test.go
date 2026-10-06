package page

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func create(t *testing.T, s *store.Store, kind store.DocumentKind, id store.ID) string {
	t.Helper()
	v, err := scaffold.Create(s, kind, id, version.Version{}, false)
	if err != nil {
		t.Fatal(err)
	}
	return s.Path(kind, id, v)
}

func TestWorkflow(t *testing.T) {
	s := store.New(t.TempDir())
	wf := create(t, s, store.Workflow, "deploy")
	task := create(t, s, store.Task, "build")

	var out bytes.Buffer
	if err := Workflow(&out, wf, nil, nil, render.Scope{}, ""); err != nil {
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
	if err := Task(&out, task, nil); err != nil {
		t.Fatalf("task: %v", err)
	}
	page = out.String()
	if !strings.Contains(page, "<h1>build</h1>") || strings.Contains(page, `class="fsm"`) {
		t.Error("task page is not titled by its id, or draws a graph")
	}
}

func TestInvalid(t *testing.T) {
	s := store.New(t.TempDir())
	create(t, s, store.Workflow, "deploy")
	bad := create(t, s, store.Workflow, "deploy")
	data, err := os.ReadFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, bytes.Replace(data, []byte("start: start"), []byte("start: nowhere"), 1), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Workflow(&out, bad, nil, nil, render.Scope{}, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v, want %v", err, ErrInvalid)
	}
	page := out.String()
	if !strings.Contains(page, "<h1>deploy</h1>") || !strings.Contains(page, `class="overlay"`) || !strings.Contains(page, "nowhere") {
		t.Error("invalid page has no header, overlay or validate error")
	}
}
