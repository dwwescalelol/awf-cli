package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestValidateStored(t *testing.T) {
	project(t)
	wf := versioned(t, store.Workflow, "deploy")
	task := versioned(t, store.Task, "build")

	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Fatalf("fresh workflow: %v", err)
	}
	if _, _, err := validate(request{kind: store.Task, args: []string{"build"}}); err != nil {
		t.Fatalf("fresh task: %v", err)
	}

	rewrite(t, wf, "name: deploy", "name: other")
	rewrite(t, wf, "version: 0.1.0\nsha", "version: 3.0.0\nsha")
	_, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}})
	if !errors.Is(err, store.ErrWrongName) || !errors.Is(err, store.ErrWrongVersion) {
		t.Errorf("workflow: got %v, want both mismatches", err)
	}
	var problems load.Problems
	if !errors.As(err, &problems) || len(problems) != 2 {
		t.Fatalf("workflow: got %v, want two problems", err)
	}
	rel := filepath.Join(".awf", store.Local, store.Workflow.Dir(), "deploy", "0.1.0.yaml")
	for i, at := range [][2]int{{2, 7}, {3, 10}} {
		if p := problems[i]; p.File != rel || p.Line != at[0] || p.Column != at[1] {
			t.Errorf("got %s:%d:%d, want %s:%d:%d", p.File, p.Line, p.Column, rel, at[0], at[1])
		}
	}
	if want := rel + `:2:7: error: name "other", stored as "deploy": ` + store.ErrWrongName.Error(); problems[0].Error() != want {
		t.Errorf("got %q, want %q", problems[0].Error(), want)
	}
	if _, _, err := validate(request{kind: store.Workflow, file: wf}); errors.Is(err, store.ErrWrongName) || errors.Is(err, store.ErrWrongVersion) {
		t.Errorf("-f skips the check: got %v", err)
	}

	rewrite(t, task, "version: 0.1.0", "version: 2.0.0")
	if _, _, err := validate(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, store.ErrWrongVersion) {
		t.Errorf("task: got %v, want %v", err, store.ErrWrongVersion)
	}
}

func versioned(t *testing.T, kind store.DocumentKind, id store.ID) string {
	t.Helper()
	data, err := scaffold.Blank(kind, id, scaffold.FirstVersion)
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(".awf")
	if err := s.Write(kind, id, scaffold.FirstVersion, data); err != nil {
		t.Fatal(err)
	}
	p, err := filepath.Abs(s.Path(kind, id, scaffold.FirstVersion))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func rewrite(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s has no %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}
