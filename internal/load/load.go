package load

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

func Task(path string) (*manifest.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateTask(front, body); err != nil {
		return nil, err
	}
	return manifest.UnmarshalTask(data)
}

// Workflow reads a workflow, inlines its $ref tasks and compiles it. Relative
// $ref paths resolve against the workflow's directory, and ids resolve in s.
func Workflow(path string, s *store.Store) (*awf.Workflow, error) {
	_, w, err := parse(path, s)
	return w, err
}

// Document reads a workflow and inlines its $ref tasks, returning the
// document only when it also compiles.
func Document(path string, s *store.Store) (*manifest.Workflow, error) {
	doc, _, err := parse(path, s)
	return doc, err
}

func parse(path string, s *store.Store) (*manifest.Workflow, *awf.Workflow, error) {
	doc, err := read(path)
	if err != nil {
		return nil, nil, err
	}
	unresolved := bundle(doc, filepath.Dir(path), s)
	w, err := compile(doc)
	if err := errors.Join(unresolved, err); err != nil {
		return nil, nil, err
	}
	return doc, w, nil
}

func read(path string) (*manifest.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := schema.Validate(data); err != nil {
		return nil, err
	}
	return manifest.Unmarshal(data)
}
