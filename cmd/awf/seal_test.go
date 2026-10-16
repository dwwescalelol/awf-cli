package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestSealWorkflow(t *testing.T) {
	project(t)
	task, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := create(store.Workflow, "deploy", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, wf.path, "start: start", "start: build")
	rewrite(t, wf.path, "  start: null", "  build: null")
	data, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(data), "tasks:")
	if err := os.WriteFile(wf.path, []byte(head+"tasks:\n  build:\n    $ref: build@0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, ErrUnsealedTasks) || !strings.Contains(err.Error(), "build") {
		t.Fatalf("seal over an unsealed task: got %v, want %v naming build", err, ErrUnsealedTasks)
	}
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); err != nil {
		t.Fatalf("seal task: %v", err)
	}
	if f, err := load.ReadTask(task.path, nil); err != nil || f.Doc.SHA == nil {
		t.Fatalf("task after seal: got %+v, %v, want a sha", f, err)
	}
	path, sha, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if path != wf.path || !strings.HasPrefix(sha, "sha256-") {
		t.Errorf("got %s %s", path, sha)
	}
	sealed, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := manifest.Unmarshal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SHA == nil || *doc.SHA != sha || doc.Tasks["build"].Task == nil || doc.Tasks["build"].Task.Source != "build@0.1.0" || doc.Tasks["build"].Task.SHA == nil {
		t.Errorf("sealed:\n%s", sealed)
	}
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Fatalf("validate sealed: %v", err)
	}

	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrSealed) {
		t.Errorf("re-seal: got %v, want %v", err, seal.ErrSealed)
	}

	rewrite(t, wf.path, "name: deploy\nversion: 0.1.0", "name: deploy\nversion: 0.2.0")
	if _, _, err := validate(request{kind: store.Workflow, file: wf.path}); err != nil {
		t.Errorf("version is not hashed: %v", err)
	}
	rewrite(t, wf.path, "name: deploy\nversion: 0.2.0", "name: deploy\nversion: 0.1.0")

	rewrite(t, wf.path, "name: deploy", "name: deploy\nsummary: edited")
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, seal.ErrMismatch)
	}
	if _, err := bundle(request{kind: store.Workflow, args: []string{"deploy"}}, ""); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("bundle edited: got %v, want %v", err, seal.ErrMismatch)
	}
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrSealed) {
		t.Errorf("re-seal edited: got %v, want %v", err, seal.ErrSealed)
	}
}

func TestSealTask(t *testing.T) {
	project(t)
	task, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); err != nil {
		t.Fatalf("seal: %v", err)
	}
	f, err := load.ReadTask(task.path, nil)
	if err != nil || f.Doc.SHA == nil || f.Doc.OpenAWF.String() != "0.1.0" {
		t.Fatalf("sealed task: got %+v, %v", f, err)
	}
	if _, _, err := validate(request{kind: store.Task, args: []string{"build"}}); err != nil {
		t.Fatalf("validate sealed: %v", err)
	}
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, seal.ErrSealed) {
		t.Errorf("re-seal: got %v, want %v", err, seal.ErrSealed)
	}

	data, err := os.ReadFile(task.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(task.path, append(data, "more\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validate(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, seal.ErrMismatch)
	}

	wf, err := create(store.Workflow, "deploy", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, wf.path, "start: start", "start: build")
	rewrite(t, wf.path, "  start: null", "  build: null")
	wfData, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(wfData), "tasks:")
	if err := os.WriteFile(wf.path, []byte(head+"tasks:\n  build:\n    $ref: build@0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("seal over an edited task: got %v, want %v", err, seal.ErrMismatch)
	}
}

func TestSealFile(t *testing.T) {
	root := project(t)
	path := filepath.Join(root, "wf.yaml")
	doc := "openawf: 0.1.0\nname: wf\nsha: null\nstart: a\norchestration:\n  a: null\ntasks:\n  a:\n    body: a\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sealDocument(request{kind: store.Workflow, file: path}); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, _, err := validate(request{kind: store.Workflow, file: path}); err != nil {
		t.Errorf("validate: %v", err)
	}
	rewrite(t, path, "body: a", "body: b")
	if _, _, err := validate(request{kind: store.Workflow, file: path}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, seal.ErrMismatch)
	}
}

func installed(t *testing.T, root string, kind store.DocumentKind, name, doc string) {
	t.Helper()
	ext := ".yaml"
	if kind == store.Task {
		ext = ".md"
	}
	dir := filepath.Join(root, ".awf", "acme", kind.Dir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0.1.0"+ext), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSealReadOnly(t *testing.T) {
	root := project(t)
	installed(t, root, store.Task, "build", "---\nopenawf: 0.1.0\nversion: 0.1.0\n---\n\n# build\n")

	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"acme#build"}}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("seal: got %v, want %v", err, store.ErrReadOnly)
	}
	if _, _, err := validate(request{kind: store.Task, args: []string{"acme#build"}}); err != nil {
		t.Errorf("validate: %v", err)
	}
}
