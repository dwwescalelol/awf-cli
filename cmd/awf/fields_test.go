package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/goccy/go-yaml"
)

const fieldsTask = `---
openawf: 0.1.0
version: 0.1.0
external-version: ext-3
summary: Build the branch.
metadata:
  icon: ./build.svg
  zeta: null
license: MIT
input:
  type: object
  properties:
    branch:
      type: string
model: anthropic/claude-sonnet-5
effort: high
max-turns: 5
timeout: 30000
outcomes:
  ok:
    type: object
  fail:
    type: object
system: |
  You build.
x-runner:
  z: 1
  a: 2
x-claude: null
sha: null
---

# build
`

const fieldsWorkflow = `openawf: 0.1.0
name: deploy
summary: Build and check.
model:
  - anthropic/claude-opus-5
  - openai/gpt-6
effort: medium
metadata:
  tags:
    - ops
  authors:
    - Ada
  zeta: null
license: Apache-2.0
version: 0.1.0
sha: null
external-version: n8n-41
start: build
orchestration:
  build:
    fail:
      task: build
      retries: 1
      session: resume
      x-runner:
        z: again
        a: now
    ok: check
  check: null
tasks:
  build:
    $ref: build@0.1.0
  check:
    max-turns: 3
    timeout: 1000
    uses:
      - gh
    system: You check.
    body: |
      # check
    x-zed: {}
    x-alpha: null
mcp:
  gh:
    type: http
    url: https://example.com/mcp
    x-runner:
      z: 1
      a: 2
x-runner:
  mode: plan
  b: 1
  a: 2
x-claude:
  permission-mode: plan
`

// keys lists the keys of an ordered mapping in the order they are written.
func keys(t *testing.T, value any) []string {
	t.Helper()
	var items yaml.MapSlice
	switch v := value.(type) {
	case manifest.Extensions:
		items = yaml.MapSlice(v)
	case manifest.Metadata:
		items = yaml.MapSlice(v)
	case yaml.MapSlice:
		items = v
	default:
		t.Fatalf("%T is not an ordered mapping", value)
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i], _ = item.Key.(string)
	}
	return out
}

func extension(t *testing.T, e manifest.Extensions, key string) any {
	t.Helper()
	value, ok := e.Value(key)
	if !ok {
		t.Fatalf("%s: missing from %v", key, e)
	}
	return value
}

func checkTaskFields(t *testing.T, task *manifest.Task) {
	t.Helper()
	if task.ExternalVersion != "ext-3" || task.License != "MIT" || task.Effort != manifest.High || task.MaxTurns != 5 || task.Timeout != 30000 {
		t.Errorf("build fields: got %+v", task)
	}
	if task.System != "You build.\n" || !slices.Equal(task.Model, manifest.Models{"anthropic/claude-sonnet-5"}) {
		t.Errorf("build system and model: got %q %q", task.System, task.Model)
	}
	if got := keys(t, task.Metadata); !slices.Equal(got, []string{"icon", "zeta"}) || task.Metadata.Icon() != "./build.svg" {
		t.Errorf("build metadata: got %v", task.Metadata)
	}
	if got := keys(t, task.Extensions); !slices.Equal(got, []string{"x-runner", "x-claude"}) {
		t.Errorf("build extensions: got %q", got)
	}
	if got := keys(t, extension(t, task.Extensions, "x-runner")); !slices.Equal(got, []string{"z", "a"}) {
		t.Errorf("build x-runner: got %q, want z then a", got)
	}
	if v := extension(t, task.Extensions, "x-claude"); v != nil {
		t.Errorf("build x-claude: got %v, want null", v)
	}
}

func checkWorkflowFields(t *testing.T, data []byte) {
	t.Helper()
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if wf.Effort != manifest.Medium || wf.License != "Apache-2.0" || wf.ExternalVersion != "n8n-41" || !slices.Equal(wf.Model, manifest.Models{"anthropic/claude-opus-5", "openai/gpt-6"}) {
		t.Errorf("workflow fields: got %+v", wf)
	}
	if got := keys(t, wf.Metadata); !slices.Equal(got, []string{"tags", "authors", "zeta"}) {
		t.Errorf("workflow metadata: got %q", got)
	}
	if got := keys(t, wf.Extensions); !slices.Equal(got, []string{"x-runner", "x-claude"}) {
		t.Errorf("workflow extensions: got %q", got)
	}
	if got := keys(t, extension(t, wf.Extensions, "x-runner")); !slices.Equal(got, []string{"mode", "b", "a"}) {
		t.Errorf("workflow x-runner: got %q", got)
	}
	fail := wf.Orchestration["build"].Branch["fail"]
	if fail.Session != manifest.Resume || fail.Retries == nil || *fail.Retries != 1 {
		t.Errorf("fail edge: got %+v", fail)
	}
	if got := keys(t, extension(t, fail.Extensions, "x-runner")); !slices.Equal(got, []string{"z", "a"}) {
		t.Errorf("fail edge x-runner: got %q", got)
	}
	if got := keys(t, extension(t, wf.MCP["gh"].Extensions, "x-runner")); !slices.Equal(got, []string{"z", "a"}) {
		t.Errorf("gh x-runner: got %q", got)
	}
	check := wf.Tasks["check"].Task
	if check == nil || check.System != "You check." || check.MaxTurns != 3 || check.Timeout != 1000 {
		t.Fatalf("check: got %+v", check)
	}
	if got := keys(t, check.Extensions); !slices.Equal(got, []string{"x-zed", "x-alpha"}) {
		t.Errorf("check extensions: got %q", got)
	}
	build := wf.Tasks["build"].Task
	if build == nil {
		t.Fatalf("build: got %+v, want an inline task", wf.Tasks["build"])
	}
	checkTaskFields(t, build)
}

func TestFieldsRoundTrip(t *testing.T) {
	root := project(t)
	task, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(task.path, []byte(fieldsTask), 0o644); err != nil {
		t.Fatal(err)
	}
	taskPath, _, err := sealDocument(request{kind: store.Task, args: []string{"build"}})
	if err != nil {
		t.Fatalf("seal task: %v", err)
	}
	sealedTask, err := load.ReadTask(taskPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkTaskFields(t, sealedTask.Doc)

	wf, err := create(store.Workflow, "deploy", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wf.path, []byte(fieldsWorkflow), 0o644); err != nil {
		t.Fatal(err)
	}
	bundled, err := bundle(request{kind: store.Workflow, args: []string{"deploy"}}, "")
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	checkWorkflowFields(t, bundled)
	if !strings.HasSuffix(string(bundled), "x-runner:\n  mode: plan\n  b: 1\n  a: 2\nx-claude:\n  permission-mode: plan\n") {
		t.Errorf("bundled extensions not as written:\n%s", bundled)
	}

	path, _, err := sealDocument(request{kind: store.Workflow, args: []string{"deploy"}})
	if err != nil {
		t.Fatalf("seal workflow: %v", err)
	}
	sealedWorkflow, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkWorkflowFields(t, sealedWorkflow)

	i, err := importDocument(request{kind: store.Workflow, args: []string{"deploy", "copy"}})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	imported, err := os.ReadFile(i.path)
	if err != nil {
		t.Fatal(err)
	}
	checkWorkflowFields(t, imported)

	out := filepath.Join(root, "out", "deploy.yaml")
	if _, err := exportDocument(request{kind: store.Workflow, args: []string{"deploy", out}}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if exported, err := os.ReadFile(out); err != nil || string(exported) != string(sealedWorkflow) {
		t.Errorf("export: got %s, %v, want a copy of the sealed workflow", exported, err)
	}

	ti, err := importDocument(request{kind: store.Task, args: []string{"build", "compile"}})
	if err != nil {
		t.Fatalf("import task: %v", err)
	}
	draft, err := load.ReadTask(ti.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkTaskFields(t, draft.Doc)
}
