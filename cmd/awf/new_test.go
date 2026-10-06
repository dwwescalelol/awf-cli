package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(store.EnvHome, filepath.Join(root, "home"))
	t.Chdir(root)
	return root
}

func TestCreateWorkflow(t *testing.T) {
	project(t)

	firstDraft, err := create(store.Workflow, "feat-dev", false)
	first := firstDraft.path
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if filepath.Base(first) != "0.1.0.yaml" {
		t.Errorf("first: got %s, want 0.1.0.yaml", first)
	}
	if _, err := load.ReadWorkflow(first, nil, nil); err != nil {
		t.Errorf("blank workflow does not load: %v", err)
	}

	secondDraft, err := create(store.Workflow, "feat-dev", false)
	second := secondDraft.path
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if filepath.Base(second) != "0.2.0.yaml" {
		t.Errorf("second: got %s, want 0.2.0.yaml", second)
	}
	data, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if wf.Version.String() != "0.2.0" || wf.SHA != nil {
		t.Errorf("second: got version %s sha %v, want 0.2.0 and null", wf.Version, wf.SHA)
	}
}

func TestCreateCopiesPrevious(t *testing.T) {
	project(t)

	firstDraft, err := create(store.Workflow, "feat-dev", false)
	first := firstDraft.path
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	sha := "sha256-abc"
	wf.Summary, wf.SHA = "Edited.", &sha
	edited, err := manifest.Marshal(wf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	secondDraft, err := create(store.Workflow, "feat-dev", false)
	second := secondDraft.path
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Summary != "Edited." {
		t.Errorf("summary: got %q, want Edited.", copied.Summary)
	}
	if copied.SHA != nil {
		t.Errorf("sha: got %q, want null", *copied.SHA)
	}
}

func TestCreateTask(t *testing.T) {
	project(t)

	pathDraft, err := create(store.Task, "create-diff", false)
	path := pathDraft.path
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	task, err := manifest.UnmarshalTask(data)
	if err != nil {
		t.Fatalf("UnmarshalTask: %v", err)
	}
	if task.Version.String() != "0.1.0" || task.SHA != nil || task.Body == "" {
		t.Errorf("got %+v", task)
	}
}

func TestCreatePinned(t *testing.T) {
	project(t)

	pathDraft, err := create(store.Workflow, "feat-dev@0.4.0", false)
	path := pathDraft.path
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if filepath.Base(path) != "0.4.0.yaml" {
		t.Errorf("got %s, want 0.4.0.yaml", path)
	}
	if _, err := create(store.Workflow, "feat-dev@0.4.0", false); !errors.Is(err, store.ErrExists) {
		t.Errorf("again: got %v, want %v", err, store.ErrExists)
	}
}

func TestCreateRejects(t *testing.T) {
	project(t)

	if _, err := create(store.Workflow, "Bad Name", false); !errors.Is(err, store.ErrNotAnID) {
		t.Errorf("bad id: got %v, want %v", err, store.ErrNotAnID)
	}
	if _, err := create(store.Workflow, "feat-dev@one", false); err == nil {
		t.Error("bad version: got no error")
	}
}

func TestCreateGlobal(t *testing.T) {
	root := project(t)

	pathDraft, err := create(store.Task, "create-diff", true)
	path := pathDraft.path
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "home", "task", "create-diff", "0.1.0.md")
	if path != want {
		t.Errorf("got %s, want %s", path, want)
	}
}

func TestCreateUnreadable(t *testing.T) {
	root := project(t)
	if _, err := create(store.Task, "create-diff@0.3.0", false); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".awf", "task", "create-diff")
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := create(store.Task, "create-diff", false); err == nil {
		t.Error("got no error, want the read error")
	}
	if _, err := os.Stat(filepath.Join(dir, "0.1.0.md")); err == nil {
		t.Error("0.1.0.md was created over an unreadable id")
	}
}
