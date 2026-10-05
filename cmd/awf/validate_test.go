package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestValidateStored(t *testing.T) {
	project(t)
	wfDraft, err := create(store.Workflow, "deploy", "", false)
	wf := wfDraft.path
	if err != nil {
		t.Fatal(err)
	}
	taskDraft, err := create(store.Task, "build", "", false)
	task := taskDraft.path
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := validate([]string{"deploy"}, "", false, false); err != nil {
		t.Fatalf("fresh workflow: %v", err)
	}
	if _, _, err := validate([]string{"build"}, "", true, false); err != nil {
		t.Fatalf("fresh task: %v", err)
	}

	rewrite(t, wf, "name: deploy", "name: other")
	rewrite(t, wf, "version: 0.1.0\nsha", "version: 3.0.0\nsha")
	_, _, err = validate([]string{"deploy"}, "", false, false)
	if !errors.Is(err, store.ErrWrongName) || !errors.Is(err, store.ErrWrongVersion) {
		t.Errorf("workflow: got %v, want both mismatches", err)
	}
	if _, _, err := validate(nil, wf, false, false); err != nil {
		t.Errorf("-f skips the check: got %v", err)
	}

	rewrite(t, task, "version: 0.1.0", "version: 2.0.0")
	if _, _, err := validate([]string{"build"}, "", true, false); !errors.Is(err, store.ErrWrongVersion) {
		t.Errorf("task: got %v, want %v", err, store.ErrWrongVersion)
	}
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
