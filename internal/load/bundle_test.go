package load

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func TestDocument(t *testing.T) {
	doc, err := Document(filepath.Join("testdata", "ref.yaml"), nil)
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	diff := doc.Tasks["diff"]
	if diff.Ref != "" || diff.Task == nil {
		t.Fatalf("diff: got %+v, want an inline task", diff)
	}
	if diff.Task.Summary != "Produce a diff for the branch." || diff.Task.Body == "" {
		t.Errorf("diff: got %+v", diff.Task)
	}
	if diff.Task.OpenAWF != (version.Version{}) {
		t.Errorf("openawf: got %s, want omitted", diff.Task.OpenAWF)
	}
}

func TestWorkflowRef(t *testing.T) {
	w, err := Workflow(filepath.Join("testdata", "ref.yaml"), nil)
	if err != nil {
		t.Fatalf("Workflow: %v", err)
	}
	if len(w.Start.Out) != 2 {
		t.Errorf("diff: got %d edges, want ok and fail", len(w.Start.Out))
	}
}

func TestUnresolvedRef(t *testing.T) {
	path := filepath.Join("testdata", "badref.yaml")
	for name, load := range map[string]func() error{
		"Document": func() error { _, err := Document(path, nil); return err },
		"Workflow": func() error { _, err := Workflow(path, nil); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := load()
			for _, want := range []error{ErrUnresolvedRef, fs.ErrNotExist, ref.ErrUnpinned} {
				if !errors.Is(err, want) {
					t.Errorf("got %v, want %v", err, want)
				}
			}
		})
	}
}
