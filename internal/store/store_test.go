package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

// setup builds a temp tree, points the home directory at it, and chdirs into
// one of its subdirectories.
func setup(t *testing.T, dirs []string, wd string) (root, home string) {
	t.Helper()
	root = t.TempDir()
	home = filepath.Join(root, "home")
	for _, dir := range append(dirs, "home") {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(EnvHome, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(filepath.Join(root, wd))
	return root, home
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		dirs   []string
		wd     string
		global bool
		env    string
		want   string // relative to root, or "" for the global scope
	}{
		{
			name: "in the current directory",
			dirs: []string{"proj/.awf"},
			wd:   "proj",
			want: "proj/.awf",
		},
		{
			name: "in an ancestor",
			dirs: []string{"proj/.awf", "proj/a/b"},
			wd:   "proj/a/b",
			want: "proj/.awf",
		},
		{
			name: "an ancestor above the project",
			dirs: []string{".awf", "proj/a"},
			wd:   "proj/a",
			want: ".awf",
		},
		{
			name:   "global forced inside a project",
			dirs:   []string{"proj/.awf"},
			wd:     "proj",
			global: true,
			want:   "",
		},
		{
			name: "project under home",
			dirs: []string{"home/proj/.awf", "home/proj/a"},
			wd:   "home/proj/a",
			want: "home/proj/.awf",
		},
		{
			name: "under home without a project",
			dirs: []string{"home/.awf", "home/loose"},
			wd:   "home/loose",
			want: "",
		},
		{
			name: "under home without a project, AWF_HOME set",
			dirs: []string{"home/.awf", "home/loose", "elsewhere"},
			wd:   "home/loose",
			env:  "elsewhere",
			want: "elsewhere",
		},
		{
			name: "outside a project",
			dirs: []string{"loose"},
			wd:   "loose",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, home := setup(t, tt.dirs, tt.wd)
			if tt.env != "" {
				t.Setenv(EnvHome, filepath.Join(root, tt.env))
			}
			want := filepath.Join(home, ".awf")
			if tt.want != "" {
				want = filepath.Join(root, tt.want)
			}

			got, err := Resolve(tt.global)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if resolve(t, got.Dir()) != resolve(t, want) {
				t.Errorf("got %q, want %q", got.Dir(), want)
			}
		})
	}
}

// A temp directory can sit behind a symlink, which os.Getwd resolves and
// t.TempDir does not.
func resolve(t *testing.T, path string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return out
}

func TestResolveCreatesNothing(t *testing.T) {
	root, home := setup(t, []string{"loose"}, "loose")
	s, err := Resolve(false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Errorf("%s exists after Resolve", s.Dir())
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("home holds %v", entries)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "loose")); len(entries) != 0 {
		t.Errorf("working directory holds %v", entries)
	}
}

func TestPath(t *testing.T) {
	s := New(filepath.Join("proj", ".awf"))
	tests := []struct {
		kind DocumentKind
		want string
	}{
		{Workflow, filepath.Join("proj", ".awf", "wf", "feat-dev", "0.1.0.yaml")},
		{Task, filepath.Join("proj", ".awf", "task", "feat-dev", "0.1.0.md")},
	}
	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			if got := s.Path(tt.kind, "feat-dev", version.Version{Minor: 1}); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateAndList(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), ".awf"))

	if documents, _, err := s.List(Workflow); err != nil || documents != nil {
		t.Fatalf("empty scope: got %v, %v", documents, err)
	}

	write := func(kind DocumentKind, id ID, v string) {
		t.Helper()
		parsed, err := version.Parse(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Write(kind, id, parsed, nil); err != nil {
			t.Fatal(err)
		}
	}
	write(Workflow, "feat-dev", "0.2.0")
	write(Workflow, "feat-dev", "0.10.0")
	write(Workflow, "feat-dev", "0.1.0")
	write(Workflow, "ship", "1.0.0")
	write(Task, "create-diff", "0.1.0")

	// Empty ids and files of the wrong kind are not versions.
	if err := os.MkdirAll(filepath.Join(s.Dir(), "wf", "drafted"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "wf", "ship", "notes.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	want := []Document{
		{ID: "feat-dev", Versions: parseAll(t, "0.1.0", "0.2.0", "0.10.0")},
		{ID: "ship", Versions: parseAll(t, "1.0.0")},
	}
	got, skipped, err := s.List(Workflow)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("workflows: got %v, want %v", got, want)
	}

	wantSkipped := []Skipped{{
		Path:   filepath.Join(s.Dir(), "wf", "ship", "notes.md"),
		Reason: ErrNotADocument,
	}}
	sameSkip := func(a, b Skipped) bool { return a.Path == b.Path && errors.Is(a.Reason, b.Reason) }
	if !slices.EqualFunc(skipped, wantSkipped, sameSkip) {
		t.Errorf("skipped: got %v, want %v", skipped, wantSkipped)
	}

	wantTasks := []Document{{ID: "create-diff", Versions: parseAll(t, "0.1.0")}}
	got, _, err = s.List(Task)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, wantTasks) {
		t.Errorf("tasks: got %v, want %v", got, wantTasks)
	}
}

func TestNewID(t *testing.T) {
	bad := []string{"", ".", "..", "a/b", "../etc", "wf/", string(filepath.Separator), "Bad Name", "feat_dev", "-feat", "feat-", "FeatDev"}
	for _, in := range bad {
		if got, err := NewID(in); err == nil {
			t.Errorf("NewID(%q): got %q, want an error", in, got)
		}
	}
	if got, err := NewID("feat-dev"); err != nil || got != ID("feat-dev") {
		t.Errorf("NewID(\"feat-dev\"): got %q, %v", got, err)
	}
}

func parseAll(t *testing.T, in ...string) []version.Version {
	t.Helper()
	out := make([]version.Version, 0, len(in))
	for _, s := range in {
		v, err := version.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}
