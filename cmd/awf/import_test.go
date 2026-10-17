package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

const shipWorkflow = "openawf: 0.1.0\nname: ship\nversion: 0.1.0\nsha: null\nstart: a\norchestration:\n  a: null\ntasks:\n  a:\n    body: a\n"

func TestImport(t *testing.T) {
	root := project(t)
	installed(t, root, store.Task, "build", "---\nopenawf: 0.1.0\nversion: 0.1.0\n---\n\n# build\n")
	installed(t, root, store.Workflow, "ship", shipWorkflow)
	for _, r := range []request{{kind: store.Task, file: filepath.Join(root, ".awf", "acme", "task", "build", "0.1.0.md")}, {kind: store.Workflow, file: filepath.Join(root, ".awf", "acme", "workflow", "ship", "0.1.0.yaml")}} {
		if _, _, err := sealDocument(r); err != nil {
			t.Fatal(err)
		}
	}

	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		r      request
		id     string
		ver    string
		sealed bool
	}{
		{name: "task", r: request{kind: store.Task, args: []string{"acme#build"}}, id: "build", ver: "0.1.0", sealed: true},
		{name: "renamed task", r: request{kind: store.Task, args: []string{"acme#build", "compile@0.2.0"}}, id: "compile", ver: "0.2.0", sealed: true},
		{name: "workflow", r: request{kind: store.Workflow, args: []string{"acme#ship@0.1.0"}}, id: "ship", ver: "0.1.0", sealed: true},
		{name: "workflow version", r: request{kind: store.Workflow, args: []string{"acme#ship", "ship@0.2.0"}}, id: "ship", ver: "0.2.0", sealed: true},
		{name: "renamed workflow", r: request{kind: store.Workflow, args: []string{"acme#ship", "deploy"}}, id: "deploy", ver: "0.1.0"},
		{name: "file", r: request{kind: store.Task, file: filepath.Join(root, ".awf", "acme", "task", "build", "0.1.0.md"), args: []string{"package@1.0.0"}}, id: "package", ver: "1.0.0", sealed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, err := importDocument(tt.r)
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(real, ".awf", store.Local, tt.r.kind.Dir(), tt.id, tt.ver+filepath.Ext(i.path))
			if i.id.String() != tt.id || i.version.String() != tt.ver || i.sealed != tt.sealed || i.path != want {
				t.Errorf("got %+v, want %s@%s sealed %v at %s", i, tt.id, tt.ver, tt.sealed, want)
			}
			if _, _, err := validate(request{kind: tt.r.kind, args: []string{tt.id + "@" + tt.ver}}); err != nil {
				t.Errorf("validate: %v", err)
			}
		})
	}
}

func TestImportBundles(t *testing.T) {
	root := project(t)
	if _, err := create(store.Task, "build", false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "wf.yaml")
	doc := "openawf: 0.1.0\nname: wf\nsha: null\nstart: build\norchestration:\n  build: null\ntasks:\n  build:\n    $ref: build@0.1.0\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	i, err := importDocument(request{kind: store.Workflow, file: path})
	if err != nil {
		t.Fatal(err)
	}
	f, err := load.ReadWorkflow(i.path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.Doc.Name != "wf" || i.version.String() != "0.1.0" || f.Doc.Tasks["build"].Task == nil || f.Doc.Tasks["build"].Task.Source != "build@0.1.0" {
		t.Errorf("got %+v", f.Doc)
	}
}

func TestImportRejects(t *testing.T) {
	root := project(t)
	installed(t, root, store.Task, "build", "---\nopenawf: 0.1.0\nversion: 0.1.0\n---\n\n# build\n")

	if _, err := importDocument(request{kind: store.Task, args: []string{"acme#build", "other#build"}}); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("foreign destination: got %v, want %v", err, store.ErrReadOnly)
	}
	if _, err := importDocument(request{kind: store.Task, args: []string{"acme#build"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := importDocument(request{kind: store.Task, args: []string{"acme#build"}}); !errors.Is(err, store.ErrExists) {
		t.Errorf("again: got %v, want %v", err, store.ErrExists)
	}
	if _, err := importDocument(request{kind: store.Task, args: []string{"acme#missing"}}); !errors.Is(err, store.ErrNotInstalled) {
		t.Errorf("missing: got %v, want %v", err, store.ErrNotInstalled)
	}
	path := filepath.Join(root, "Bad Name.md")
	if err := os.WriteFile(path, []byte("---\nopenawf: 0.1.0\nversion: 0.1.0\n---\n\n# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := importDocument(request{kind: store.Task, file: path}); !errors.Is(err, errNoName) {
		t.Errorf("unnamed file: got %v, want %v", err, errNoName)
	}
}
