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

type WorkflowFile struct {
	Source   []byte
	Doc      *manifest.Workflow
	Compiled *awf.Workflow
}

type TaskFile struct {
	Source []byte
	Doc    *manifest.Task
}

func Task(path string) (*manifest.Task, error) {
	f, err := ReadTask(path)
	if err != nil {
		return nil, err
	}
	return f.Doc, nil
}

// Workflow reads a workflow, inlines its $ref tasks and compiles it. Relative
// $ref paths resolve against the workflow's directory, and ids resolve in s.
func Workflow(path string, s *store.Store) (*awf.Workflow, error) {
	f, err := ReadWorkflow(path, s)
	if err != nil {
		return nil, err
	}
	return f.Compiled, nil
}

// Document reads a workflow and inlines its $ref tasks, returning the
// document only when it also compiles.
func Document(path string, s *store.Store) (*manifest.Workflow, error) {
	f, err := ReadWorkflow(path, s)
	if err != nil {
		return nil, err
	}
	return f.Doc, nil
}

func ReadTask(path string) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &TaskFile{Source: data}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return f, err
	}
	invalid := schema.ValidateTask(front, body)
	if f.Doc, err = manifest.UnmarshalTask(data); err != nil {
		f.Doc = nil
		if invalid == nil {
			invalid = err
		}
	}
	return f, invalid
}

func ReadWorkflow(path string, s *store.Store) (*WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &WorkflowFile{Source: data}
	_, invalid := schema.Validate(data)
	if f.Doc, err = manifest.Unmarshal(data); err != nil {
		f.Doc = nil
		if invalid == nil {
			invalid = err
		}
	}
	if invalid != nil {
		return f, invalid
	}
	unresolved := bundle(f.Doc, filepath.Dir(path), s)
	w, err := compile(f.Doc)
	if err := errors.Join(unresolved, err); err != nil {
		return f, err
	}
	f.Compiled = w
	return f, nil
}
