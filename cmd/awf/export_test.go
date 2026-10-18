package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	e, err := exportDocument(request{kind: store.Workflow, args: []string{"deploy", out}}, formatAWF)
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
	if _, err := exportDocument(request{kind: store.Workflow, file: out, args: []string{again}}, formatAWF); err != nil {
		t.Errorf("-f: %v", err)
	}
	if _, err := exportDocument(request{kind: store.Task, args: []string{"acme#build", filepath.Join(root, "build.md")}}, formatAWF); err != nil {
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
		if _, err := exportDocument(tt.r, formatAWF); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestExportClaude(t *testing.T) {
	root := project(t)
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
	doc := head + "tasks:\n  build:\n    $ref: build@0.1.0\nmcp:\n  github:\n    transport: http\n    tools:\n      - create_pr\n  k8s:\n    transport: stdio\n    tools: \"*\"\n"
	if err := os.WriteFile(wf.path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	stamp(t, wf.path, "0.1.0")
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}}); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "plugins", "deploy")
	e, err := exportDocument(request{kind: store.Workflow, args: []string{"deploy", out}}, formatClaude)
	if err != nil {
		t.Fatal(err)
	}
	if e.path != out {
		t.Errorf("got %+v, want %s", e, out)
	}
	for path, want := range map[string]string{
		".claude-plugin/plugin.json":   `"name": "deploy"`,
		"skills/deploy/SKILL.md":       "allowed-tools:\n- mcp__github__create_pr\n- mcp__k8s__*\n",
		"skills/deploy/tasks/build.md": "# build",
	} {
		got, err := os.ReadFile(filepath.Join(out, path))
		if err != nil || !strings.Contains(string(got), want) {
			t.Errorf("%s: got %s, %v, want %q", path, got, err, want)
		}
	}
	if skill, _ := os.ReadFile(filepath.Join(out, "skills", "deploy", "SKILL.md")); !strings.Contains(string(skill), "| `build` | any | end | | |") {
		t.Errorf("orchestration:\n%s", skill)
	}

	tests := []struct {
		name string
		r    request
		want error
	}{
		{name: "existing", r: request{kind: store.Workflow, args: []string{"deploy", out}}, want: store.ErrExists},
		{name: "into a scope", r: request{kind: store.Workflow, args: []string{"deploy", filepath.Join(root, ".awf", "plugin")}}, want: errIntoScope},
		{name: "the scope itself", r: request{kind: store.Workflow, args: []string{"deploy", filepath.Join(root, ".awf")}}, want: store.ErrExists},
	}
	for _, tt := range tests {
		if _, err := exportDocument(tt.r, formatClaude); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
	if _, err := exportDocument(request{kind: store.Workflow, args: []string{"deploy", filepath.Join(root, "x")}}, "gha"); !errors.Is(err, errFormat) {
		t.Errorf("unknown format: got %v, want %v", err, errFormat)
	}
}
