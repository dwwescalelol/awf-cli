package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
			name: "outside a project",
			dirs: []string{"loose"},
			wd:   "loose",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, home := setup(t, tt.dirs, tt.wd)
			want := filepath.Join(home, ".awf")
			if tt.want != "" {
				want = filepath.Join(root, tt.want)
			}

			got, err := Resolve(tt.global)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if resolve(t, got.Dir) != resolve(t, want) {
				t.Errorf("got %q, want %q", got.Dir, want)
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
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Errorf("%s exists after Resolve", s.Dir)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("home holds %v", entries)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "loose")); len(entries) != 0 {
		t.Errorf("working directory holds %v", entries)
	}
}

func TestPath(t *testing.T) {
	s := Scope{Dir: filepath.Join("proj", ".awf")}
	tests := []struct {
		kind Kind
		want string
	}{
		{Workflow, filepath.Join("proj", ".awf", "wf", "feat-dev", "0.1.0.yaml")},
		{Task, filepath.Join("proj", ".awf", "task", "feat-dev", "0.1.0.md")},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			if got := s.Path(tt.kind, "feat-dev", "0.1.0"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateAndList(t *testing.T) {
	s := Scope{Dir: filepath.Join(t.TempDir(), ".awf")}

	if entries, err := s.List(Workflow); err != nil || entries != nil {
		t.Fatalf("empty scope: got %v, %v", entries, err)
	}

	write := func(kind Kind, id, version string) {
		t.Helper()
		if err := s.Mkdir(kind, id); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.Path(kind, id, version), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(Workflow, "feat-dev", "0.2.0")
	write(Workflow, "feat-dev", "0.10.0")
	write(Workflow, "feat-dev", "0.1.0")
	write(Workflow, "ship", "1.0.0")
	write(Task, "create-diff", "0.1.0")

	// Empty ids and files of the wrong kind are not versions.
	if err := s.Mkdir(Workflow, "drafted"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "wf", "ship", "notes.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	want := []Entry{
		{ID: "feat-dev", Versions: []string{"0.1.0", "0.10.0", "0.2.0"}},
		{ID: "ship", Versions: []string{"1.0.0"}},
	}
	got, err := s.List(Workflow)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("workflows: got %v, want %v", got, want)
	}

	wantTasks := []Entry{{ID: "create-diff", Versions: []string{"0.1.0"}}}
	got, err = s.List(Task)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !reflect.DeepEqual(got, wantTasks) {
		t.Errorf("tasks: got %v, want %v", got, wantTasks)
	}
}
