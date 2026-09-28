package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestBundle(t *testing.T) {
	root := project(t)
	if _, err := create(store.Task, "build", "", false); err != nil {
		t.Fatal(err)
	}
	wf, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "check.md")
	if err := os.WriteFile(local, []byte("---\nopenawf: 0.1.0\nsha: null\n---\n\n# check\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rewrite(t, wf.path, "start: start", "start: build")
	rewrite(t, wf.path, "  start: null", "  build: check\n  check: null")
	data, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(data), "tasks:")
	refs := head + "tasks:\n  build:\n    $ref: build@0.1.0\n  check:\n    $ref: " + local + "\n"
	if err := os.WriteFile(wf.path, []byte(refs), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := validate([]string{"deploy"}, "", false, false); err != nil {
		t.Fatalf("validate with refs: %v", err)
	}
	out, err := bundle([]string{"deploy"}, "", "", false)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	doc, err := manifest.Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"build", "check"} {
		if entry := doc.Tasks[name]; entry.Task == nil {
			t.Errorf("%s: got %+v, want an inline task", name, entry)
		}
	}
	if strings.Contains(string(out), "$ref") || strings.Contains(string(out), "openawf: 0.1.0\n    ") {
		t.Errorf("bundled:\n%s", out)
	}

	bundled := filepath.Join(root, "bundled.yaml")
	if err := os.WriteFile(bundled, out, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validate(nil, bundled, false, false); err != nil {
		t.Errorf("bundled output does not validate: %v", err)
	}
}

func TestBundleOut(t *testing.T) {
	root := project(t)
	wf, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(root, "loose.yaml")
	data, err := os.ReadFile(wf.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loose, data, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		file string
		out  string
		want error
	}{
		{name: "stored input", args: []string{"deploy"}, out: wf.path, want: errOutIsInput},
		{name: "file input", file: loose, out: loose, want: errOutIsInput},
		{name: "into store", file: loose, out: filepath.Join(root, ".awf", "wf", "deploy", "0.2.0.yaml"), want: errOutInStore},
		{name: "outside", args: []string{"deploy"}, out: filepath.Join(root, "out.yaml")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := bundle(tt.args, tt.file, tt.out, false); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBundleRejectsInvalidGraph(t *testing.T) {
	root := project(t)
	path := filepath.Join(root, "wf.yaml")
	doc := "openawf: 0.1.0\nname: wf\nsha: null\nstart: a\norchestration:\n  a: ghost\ntasks:\n  a:\n    body: a\n"
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle(nil, path, "", false); err == nil {
		t.Error("edge to an undefined task: got no error")
	}
}
