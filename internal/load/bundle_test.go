package load

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func TestBundle(t *testing.T) {
	f, err := ReadWorkflow(filepath.Join("testdata", "ref.yaml"), nil, nil)
	if err != nil {
		t.Fatalf("ReadWorkflow: %v", err)
	}
	diff := f.Doc.Tasks["diff"]
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
	f, err := ReadWorkflow(filepath.Join("testdata", "ref.yaml"), nil, nil)
	if err != nil {
		t.Fatalf("ReadWorkflow: %v", err)
	}
	if len(f.Compiled.Start.Out) != 2 {
		t.Errorf("diff: got %d edges, want ok and fail", len(f.Compiled.Start.Out))
	}
}

func TestUnresolvedRef(t *testing.T) {
	_, err := ReadWorkflow(filepath.Join("testdata", "badref.yaml"), nil, nil)
	for _, want := range []error{ErrUnresolvedRef, fs.ErrNotExist, ref.ErrUnpinned} {
		if !errors.Is(err, want) {
			t.Errorf("got %v, want %v", err, want)
		}
	}
}

func TestUnresolvedRefEdges(t *testing.T) {
	_, err := ReadWorkflow(filepath.Join("testdata", "branchref.yaml"), nil, nil)
	if !errors.Is(err, ErrUnresolvedRef) || !errors.Is(err, ErrNotATask) {
		t.Errorf("got %v, want the $ref and the undefined edge target", err)
	}
	if errors.Is(err, ErrNoOutcomes) {
		t.Errorf("got %v, want no outcome check on an unresolved task", err)
	}
}
