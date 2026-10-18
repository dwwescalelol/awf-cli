package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestExport(t *testing.T) {
	root := project(t)
	src := sealed(t, store.Workflow, "deploy", "0.1.0")
	installed(t, root, store.Task, "build", "---\nopenawf: 0.1.0\nversion: 0.1.0\n---\n\n# build\n")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "out", "deploy.yaml")
	e, err := exportDocument(request{kind: store.Workflow, args: []string{"deploy", out}})
	if err != nil {
		t.Fatal(err)
	}
	if e.path != out || e.from != "deploy@0.1.0" {
		t.Errorf("got %+v, want deploy@0.1.0 at %s", e, out)
	}
	if got, err := os.ReadFile(out); err != nil || string(got) != string(want) {
		t.Errorf("got %s, %v, want a copy of %s", got, err, src)
	}
	if _, _, err := validate(request{kind: store.Workflow, file: out}); err != nil {
		t.Errorf("validate export: %v", err)
	}

	again := filepath.Join(root, "again.yaml")
	if _, err := exportDocument(request{kind: store.Workflow, file: out, args: []string{again}}); err != nil {
		t.Errorf("-f: %v", err)
	}
	if _, err := exportDocument(request{kind: store.Task, args: []string{"acme#build", filepath.Join(root, "build.md")}}); err != nil {
		t.Errorf("foreign namespace: %v", err)
	}
}

func TestExportRejects(t *testing.T) {
	root := project(t)
	src := sealed(t, store.Workflow, "deploy", "0.1.0")
	if _, err := create(store.Workflow, "draft-only", false); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(root, "existing.yaml")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(filepath.Dir(src), store.Draft+".yaml")
	if err := os.WriteFile(draft, data, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, ".awf"), link); err != nil {
		t.Fatal(err)
	}
	t.Setenv(store.EnvHome, "home")

	tests := []struct {
		name string
		r    request
		want error
	}{
		{name: "draft", r: request{kind: store.Workflow, args: []string{"draft-only", filepath.Join(root, "d.yaml")}}, want: errUnsealed},
		{name: "draft with a sha", r: request{kind: store.Workflow, args: []string{"deploy", filepath.Join(root, "a.yaml")}}, want: errUnsealed},
		{name: "-f draft with a sha", r: request{kind: store.Workflow, file: draft, args: []string{filepath.Join(root, "b.yaml")}}, want: errUnsealed},
		{name: "into a scope", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", filepath.Join(root, ".awf", "deploy.yaml")}}, want: errIntoScope},
		{name: "symlink to a missing dir in a scope", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", filepath.Join(link, "new", "deploy.yaml")}}, want: errIntoScope},
		{name: "relative $AWF_HOME", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", filepath.Join(root, "home", "new", "deploy.yaml")}}, want: errIntoScope},
		{name: "existing", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", existing}}, want: store.ErrExists},
		{name: "wrong kind", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", filepath.Join(root, "deploy.md")}}, want: errWrongKind},
		{name: "not a document", r: request{kind: store.Workflow, args: []string{"deploy@0.1.0", filepath.Join(root, "deploy.txt")}}, want: store.ErrNotADocument},
	}
	for _, tt := range tests {
		if _, err := exportDocument(tt.r); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
}
