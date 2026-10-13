package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/remote"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

// publish commits a store at dir/.awf to a git repository at repo, holding a
// sealed review workflow at 0.1.0 and 0.2.0 and an unsealed task.
func publish(t *testing.T, repo, dir, task string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for range 2 {
		d, err := create(store.Workflow, "review", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := sealDocument(request{kind: store.Workflow, args: []string{"review@" + d.version.String()}}, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := create(store.Task, task, false); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=awf", "-c", "user.email=awf@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--quiet", "--initial-branch", "main")
	run("add", ".")
	run("commit", "--quiet", "-m", "publish")
}

func TestInstall(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	base := t.TempDir()
	publish(t, filepath.Join(base, "zaz", "flows"), filepath.Join(base, "zaz", "flows"), "lint")
	publish(t, filepath.Join(base, "zaz", "mono"), filepath.Join(base, "zaz", "mono", "sub"), "lint")
	publish(t, filepath.Join(base, "zaz", "both"), filepath.Join(base, "zaz", "both"), "review")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url.file://"+filepath.ToSlash(base)+"/.insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://example.com/")
	root := project(t)

	i, err := install("https://example.com/zaz/flows.git#review", "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".awf", "example.com", "zaz", "flows", store.Workflow.Dir(), "review", "0.2.0.yaml")
	if i.path != want || i.id != "example.com/zaz/flows#review" || i.existed {
		t.Errorf("latest: got %+v, want %s", i, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Error(err)
	}
	if i, err := install("example.com/zaz/flows#review@0.2.0", "", false); err != nil || !i.existed {
		t.Errorf("again: got %+v, %v, want already installed", i, err)
	}

	i, err = install("example.com/zaz/mono/sub#review@0.1.0", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	if i.id != "example.com/zaz/mono/sub#review" {
		t.Errorf("subpath: got %s", i.id)
	}
	if i, err := install("https://example.com/zaz/flows/blob/main/.awf/_/workflow/review/0.1.0.yaml", "", false); err != nil || i.id != "example.com/zaz/flows#review" || i.version.String() != "0.1.0" {
		t.Errorf("file url: got %+v, %v", i, err)
	}
	if i, err := install("https://example.com/zaz/mono/tree/main/sub#review@0.1.0", "", false); err != nil || !i.existed {
		t.Errorf("tree url: got %+v, %v, want already installed", i, err)
	}
	if i, err := install("https://example.com/zaz/flows/tree/main/.awf#review@0.1.0", "", false); err != nil || !i.existed {
		t.Errorf(".awf url: got %+v, %v, want already installed", i, err)
	}
	if i, err := install("https://example.com/zaz/flows/tree/main/.awf/_/workflow/review", "", false); err != nil || i.version.String() != "0.2.0" {
		t.Errorf("id url: got %+v, %v, want the latest", i, err)
	}
	rewrite(t, i.path, "name: review", "name: changed")
	if _, err := install("example.com/zaz/mono/sub#review@0.1.0", "", false); !errors.Is(err, errConflict) {
		t.Errorf("changed: got %v, want %v", err, errConflict)
	}

	tests := []struct {
		name   string
		arg    string
		branch string
		want   error
	}{
		{name: "no namespace", arg: "review", want: errNoNamespace},
		{name: "url without id", arg: "https://example.com/zaz/flows/tree/main", want: errNoID},
		{name: "repo without id", arg: "example.com/zaz/flows", want: errNoID},
		{name: "unsealed", arg: "example.com/zaz/flows#lint", want: errUnsealed},
		{name: "both kinds", arg: "example.com/zaz/both#review", want: errAmbiguous},
		{name: "no version", arg: "example.com/zaz/flows#review@9.0.0", want: remote.ErrNotFound},
		{name: "no id", arg: "example.com/zaz/flows#absent", want: remote.ErrNotFound},
		{name: "no branch", arg: "example.com/zaz/flows#review", branch: "nope", want: remote.ErrNoBranch},
		{name: "no repo", arg: "example.com/zaz/absent#review", want: remote.ErrNoRepo},
		{name: "no repo url", arg: "https://example.com/zaz/absent/blob/main/.awf/_/workflow/review/0.1.0.yaml", want: remote.ErrNoRepo},
		{name: "bad namespace", arg: "file:///tmp/x#review", want: store.ErrNotANamespace},
	}
	if _, err := install("example.com/zaz/flows#review@9.0.0", "", false); err == nil || !strings.Contains(err.Error(), "available 0.1.0, 0.2.0") {
		t.Errorf("no version: got %v, want the available versions", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := install(tt.arg, tt.branch, false); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}
