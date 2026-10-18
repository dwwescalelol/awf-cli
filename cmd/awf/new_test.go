package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
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

	d, err := create(store.Workflow, "feat-dev", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if filepath.Base(d.path) != "draft.yaml" {
		t.Errorf("got %s, want draft.yaml", d.path)
	}
	if _, err := load.ReadWorkflow(d.path, nil, nil); err != nil {
		t.Errorf("blank workflow does not load: %v", err)
	}
	wf := readWorkflow(t, d.path)
	if wf.Version != (version.Version{}) || wf.SHA != nil {
		t.Errorf("got version %s sha %v, want none", wf.Version, wf.SHA)
	}
	if _, err := create(store.Workflow, "feat-dev", false); !errors.Is(err, store.ErrExists) {
		t.Errorf("second draft: got %v, want %v", err, store.ErrExists)
	}
}

func TestCreateCopiesLatest(t *testing.T) {
	project(t)

	d, err := create(store.Workflow, "feat-dev", false)
	if err != nil {
		t.Fatal(err)
	}
	rewrite(t, d.path, "name: feat-dev", "name: feat-dev\nsummary: Edited.")
	stamp(t, d.path, "0.1.0")
	if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"feat-dev"}}); err != nil {
		t.Fatal(err)
	}

	d, err = create(store.Workflow, "feat-dev", false)
	if err != nil {
		t.Fatal(err)
	}
	copied := readWorkflow(t, d.path)
	if copied.Summary != "Edited." {
		t.Errorf("summary: got %q, want Edited.", copied.Summary)
	}
	if copied.Version != (version.Version{}) || copied.SHA != nil {
		t.Errorf("got version %s sha %v, want none", copied.Version, copied.SHA)
	}
}

func TestCreateTask(t *testing.T) {
	project(t)

	d, err := create(store.Task, "create-diff", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	data, err := os.ReadFile(d.path)
	if err != nil {
		t.Fatal(err)
	}
	task, err := manifest.UnmarshalTask(data)
	if err != nil {
		t.Fatalf("UnmarshalTask: %v", err)
	}
	if filepath.Base(d.path) != "draft.md" || task.Version != (version.Version{}) || task.SHA != nil || task.Body == "" {
		t.Errorf("got %s %+v", d.path, task)
	}
}

func TestCreateRejects(t *testing.T) {
	project(t)

	tests := []struct {
		arg  string
		want error
	}{
		{arg: "Bad Name", want: store.ErrNotAnID},
		{arg: "feat-dev@0.4.0", want: errNewPinned},
		{arg: "example.com/zaz/flows#feat-dev", want: store.ErrReadOnly},
		{arg: "example.com/zaz/flows#feat-dev@1.0.0", want: store.ErrReadOnly},
	}
	for _, tt := range tests {
		if _, err := create(store.Workflow, tt.arg, false); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.arg, err, tt.want)
		}
	}
	if _, err := create(store.Workflow, "acme#feat-dev", false); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("foreign id: got %v, want %v", err, store.ErrReadOnly)
	}
}

func TestCreateGlobal(t *testing.T) {
	root := project(t)

	d, err := create(store.Task, "create-diff", true)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "home", store.Local, store.Task.Dir(), "create-diff", "draft.md")
	if d.path != want {
		t.Errorf("got %s, want %s", d.path, want)
	}
}

func TestCreateUnreadable(t *testing.T) {
	root := project(t)
	sealed(t, store.Task, "create-diff", "0.1.0")
	dir := filepath.Join(root, ".awf", store.Local, store.Task.Dir(), "create-diff")
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := create(store.Task, "create-diff", false); err == nil {
		t.Error("got no error, want the read error")
	}
	if _, err := os.Stat(filepath.Join(dir, "draft.md")); err == nil {
		t.Error("draft.md was created over an unreadable id")
	}
}

func readWorkflow(t *testing.T, path string) *manifest.Workflow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return wf
}
