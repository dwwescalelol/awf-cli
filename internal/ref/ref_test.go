package ref

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Ref
	}{
		{in: "./diff.md", want: Ref{Path: "./diff.md"}},
		{in: "diff.md", want: Ref{Path: "diff.md"}},
		{in: "tasks/diff", want: Ref{Path: "tasks/diff"}},
		{in: "diff@1.2.3", want: Ref{ID: "diff", Version: version.Version{Major: 1, Minor: 2, Patch: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{in: "https://example.com/diff.md", want: ErrRemote},
		{in: "diff", want: ErrUnpinned},
		{in: "Diff@1.2.3", want: store.ErrNotAnID},
		{in: "diff@1", want: version.ErrNotAVersion},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if _, err := Parse(tt.in); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLocate(t *testing.T) {
	s := store.New(t.TempDir())
	v := version.FirstVersion
	if err := s.Write(store.Task, "diff", v, []byte("---\n---\n")); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(t.TempDir(), "diff.md")

	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{name: "relative", ref: Ref{Path: "./tasks/diff.md"}, want: filepath.Join("wf", "tasks", "diff.md")},
		{name: "absolute", ref: Ref{Path: abs}, want: abs},
		{name: "id", ref: Ref{ID: "diff", Version: v}, want: s.Path(store.Task, "diff", v)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ref.Locate("wf", s)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}

	if _, err := (Ref{ID: "absent", Version: v}).Locate("wf", s); !errors.Is(err, store.ErrNotInstalled) {
		t.Errorf("absent id: got %v, want %v", err, store.ErrNotInstalled)
	}
}
