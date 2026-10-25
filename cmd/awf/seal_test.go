package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func sealed(t *testing.T, kind store.DocumentKind, id, v string) string {
	t.Helper()
	d, err := create(kind, id, false)
	if err != nil {
		t.Fatal(err)
	}
	stamp(t, d.path, v)
	path, _, err := sealDocument(request{kind: kind, args: []string{id}})
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func stamp(t *testing.T, path, v string) {
	t.Helper()
	rewrite(t, path, "openawf: 0.1.0\n", "openawf: 0.1.0\nversion: "+v+"\n")
}

func TestSealWorkflow(t *testing.T) {
	root := project(t)
	sealed(t, store.Task, "build", "0.1.0")
	check := filepath.Join(root, "check.md")
	if err := os.WriteFile(check, []byte("---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\n---\n\n# check\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wf, err := create(store.Workflow, "deploy", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, wf.path, "start: start", "start: build")
	rewrite(t, wf.path, "  start: null", "  build: check\n  check: null")
	data, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(data), "tasks:")
	if err := os.WriteFile(wf.path, []byte(head+"tasks:\n  build:\n    $ref: build@0.1.0\n  check:\n    $ref: "+check+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp(t, wf.path, "0.1.0")

	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, ErrUnsealedTasks) || !strings.Contains(err.Error(), "check") {
		t.Fatalf("seal over an unsealed task: got %v, want %v naming check", err, ErrUnsealedTasks)
	}
	sealFile(t, store.Task, check)
	if f, err := load.ReadTask(check, nil); err != nil || f.Doc.SHA == nil {
		t.Fatalf("task after seal: got %+v, %v, want a sha", f, err)
	}
	path, sha, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if filepath.Base(path) != "0.1.0.yaml" || !strings.HasPrefix(sha, "sha256-") {
		t.Errorf("got %s %s", path, sha)
	}
	if _, err := os.Stat(wf.path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("draft after seal: got %v, want it gone", err)
	}
	doc := readWorkflow(t, path)
	if doc.SHA == nil || *doc.SHA != sha || doc.Version.String() != "0.1.0" || doc.Tasks["build"].Task == nil || doc.Tasks["build"].Task.Source != "build@0.1.0" || doc.Tasks["build"].Task.SHA == nil {
		t.Errorf("sealed: %+v", doc)
	}
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Fatalf("validate sealed: %v", err)
	}

	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, store.ErrNoDraft) {
		t.Errorf("re-seal: got %v, want %v", err, store.ErrNoDraft)
	}

	rewrite(t, path, "name: deploy\nversion: 0.1.0", "name: deploy\nversion: 0.2.0")
	if _, _, err := validate(request{kind: store.Workflow, file: path}); err != nil {
		t.Errorf("version is not hashed: %v", err)
	}
	rewrite(t, path, "name: deploy\nversion: 0.2.0", "name: deploy\nversion: 0.1.0")

	rewrite(t, path, "name: deploy", "name: deploy\nsummary: edited")
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, seal.ErrMismatch)
	}
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, store.ErrNoDraft) {
		t.Errorf("re-seal edited: got %v, want %v", err, store.ErrNoDraft)
	}
}

func TestSealDraft(t *testing.T) {
	project(t)
	d, err := create(store.Task, "build", false)
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
	if err := os.WriteFile(wf.path, []byte(head+"tasks:\n  build:\n    $ref: "+d.path+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp(t, wf.path, "0.1.0")
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, ErrUnsealedTasks) {
		t.Errorf("seal over a draft: got %v, want %v", err, ErrUnsealedTasks)
	}

	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, errNoVersion) {
		t.Errorf("a draft without a version: got %v, want %v", err, errNoVersion)
	}
	stamp(t, d.path, "0.1.0")
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build@0.1.0"}}); !errors.Is(err, errDraftPinned) {
		t.Errorf("a pinned id: got %v, want %v", err, errDraftPinned)
	}
	path, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}})
	if err != nil || filepath.Base(path) != "0.1.0.md" {
		t.Fatalf("a draft: got %s, %v, want 0.1.0.md", path, err)
	}
	if _, err := os.Stat(d.path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("draft after seal: got %v, want it gone", err)
	}
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, store.ErrNoDraft) {
		t.Errorf("no draft: got %v, want %v", err, store.ErrNoDraft)
	}
}

func TestSealVersion(t *testing.T) {
	project(t)
	sealed(t, store.Task, "build", "0.1.0")
	d, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	seal := func() (string, error) {
		path, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}})
		return path, err
	}
	if _, err := seal(); !errors.Is(err, errNoVersion) {
		t.Errorf("no version: got %v, want %v", err, errNoVersion)
	}
	for _, v := range []string{"0.1.0", "0.0.9"} {
		stamp(t, d.path, v)
		if _, err := seal(); !errors.Is(err, errNotNewer) {
			t.Errorf("%s: got %v, want %v", v, err, errNotNewer)
		}
		rewrite(t, d.path, "version: "+v+"\n", "")
	}
	stamp(t, d.path, "1.0.0")
	if path, err := seal(); err != nil || filepath.Base(path) != "1.0.0.md" {
		t.Errorf("1.0.0: got %s, %v", path, err)
	}
}

func TestSealTask(t *testing.T) {
	project(t)
	path := sealed(t, store.Task, "build", "0.1.0")
	f, err := load.ReadTask(path, nil)
	if err != nil || f.Doc.SHA == nil || f.Doc.OpenAWF.String() != "0.1.0" || f.Doc.Version.String() != "0.1.0" {
		t.Fatalf("sealed task: got %+v, %v", f, err)
	}
	if _, _, err := validate(request{kind: store.Task, args: []string{"build"}}); err != nil {
		t.Fatalf("validate sealed: %v", err)
	}
	if _, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}}); !errors.Is(err, store.ErrNoDraft) {
		t.Errorf("re-seal: got %v, want %v", err, store.ErrNoDraft)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, "more\n"...), 0o644); err != nil {
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
	stamp(t, wf.path, "0.1.0")
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("seal over an edited task: got %v, want %v", err, seal.ErrMismatch)
	}
}

func sealFile(t *testing.T, kind store.DocumentKind, path string) {
	t.Helper()
	d := document{kind: kind, path: path}
	var data []byte
	var err error
	if kind == store.Task {
		data, _, _, err = sealTask(d)
	} else {
		data, _, _, err = sealWorkflow(d)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
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
	path := filepath.Join(root, name+ext)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	sealFile(t, kind, path)
	if err := os.Rename(path, filepath.Join(dir, "0.1.0"+ext)); err != nil {
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

func TestInlineTaskSHA(t *testing.T) {
	project(t)
	sealed(t, store.Task, "build", "0.1.0")
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
	stamp(t, wf.path, "0.1.0")
	path, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importDocument(request{kind: store.Workflow, file: path, args: []string{"copy"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"copy"}}); err != nil {
		t.Fatalf("imported: %v", err)
	}
	rewrite(t, path, "# build", "# edited")
	draft := filepath.Join(filepath.Dir(filepath.Dir(path)), "copy", store.Draft+".yaml")
	rewrite(t, draft, "# build", "# edited")
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"copy"}}); !errors.Is(err, seal.ErrMismatch) {
		t.Errorf("edited inline task: got %v, want %v", err, seal.ErrMismatch)
	}
}

func TestInlineTaskOwnSource(t *testing.T) {
	project(t)
	d, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, d.path, "openawf: 0.1.0\n", "openawf: 0.1.0\nsource: https://example.com/build.md\n")
	sealed := func(kind store.DocumentKind, id string) {
		t.Helper()
		path := d.path
		if kind == store.Workflow {
			path = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(d.path))), "workflow", id, store.Draft+".yaml")
		}
		stamp(t, path, "0.1.0")
		if _, _, err := sealDocument(request{kind: kind, args: []string{id}}); err != nil {
			t.Fatal(err)
		}
	}
	sealed(store.Task, "build")
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
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Fatalf("draft: %v", err)
	}
	sealed(store.Workflow, "deploy")
	if _, _, err := validate(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Errorf("sealed: %v", err)
	}
}
