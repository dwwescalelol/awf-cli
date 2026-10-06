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
	Refs     []string
	Doc      *manifest.Workflow
	Compiled *awf.Workflow
}

type TaskFile struct {
	Source []byte
	Doc    *manifest.Task
}

func ReadTask(path string, at *store.Entry) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &TaskFile{Source: data}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return f, err
	}
	doc, invalid := manifest.TaskDocument(front, body)
	if invalid == nil {
		invalid = schema.ValidateTask(doc)
	}
	if f.Doc, err = manifest.DecodeTask(front, body); err != nil {
		f.Doc = nil
		if invalid == nil {
			invalid = err
		}
	}
	if invalid != nil {
		return f, invalid
	}
	return f, at.Check("", f.Doc.Version)
}

func ReadWorkflow(path string, s *store.Store, at *store.Entry) (*WorkflowFile, error) {
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
	refs, unresolved := bundle(f.Doc, base(path, s), s)
	f.Refs = refs
	w, err := compile(f.Doc)
	if err := errors.Join(unresolved, err); err != nil {
		return f, err
	}
	f.Compiled = w
	return f, at.Check(f.Doc.Name, f.Doc.Version)
}

// base is the directory a relative $ref resolves against: the parent of the
// store in scope, or the workflow's own directory when there is no store.
func base(path string, s *store.Store) string {
	if s != nil {
		return filepath.Dir(s.Dir())
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return filepath.Dir(path)
}
