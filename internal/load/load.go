package load

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

type WorkflowFile struct {
	Source   []byte
	Refs     []string
	Sources  map[string]string
	Doc      *manifest.Workflow
	Compiled *awf.Workflow
	Warnings Problems
	src      *source
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
	return ParseTask(path, data, at)
}

func ParseTask(path string, data []byte, at *store.Entry) (*TaskFile, error) {
	f := &TaskFile{Source: data}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return f, (&source{file: display(path)}).report([]error{err})
	}
	src, err := parse(path, front)
	if err != nil {
		return f, err
	}
	return f, src.report(f.read(front, body, at))
}

func (f *TaskFile) read(front []byte, body string, at *store.Entry) []error {
	var invalid []error
	doc, err := manifest.TaskDocument(front, body)
	switch {
	case errors.Is(err, manifest.ErrReservedBody):
		invalid = []error{onKey("body", err)}
	case err != nil:
		invalid = []error{err}
	default:
		invalid = schema.ValidateTask(doc)
	}
	if f.Doc, err = manifest.DecodeTask(front, body); err != nil {
		f.Doc = nil
		if len(invalid) == 0 {
			invalid = []error{err}
		}
	}
	if len(invalid) > 0 {
		return invalid
	}
	if err := at.CheckVersion(f.Doc.Version); err != nil {
		return []error{onValue("version", err)}
	}
	if err := seal.CheckTask(f.Doc); err != nil {
		return []error{onValue("sha", err)}
	}
	return nil
}

func ReadWorkflow(path string, s *store.Store, at *store.Entry) (*WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseWorkflow(path, data, s, at, nil)
}

type ReadFunc func(path string) ([]byte, error)

func ParseWorkflow(path string, data []byte, s *store.Store, at *store.Entry, read ReadFunc) (*WorkflowFile, error) {
	f := &WorkflowFile{Source: data}
	src, err := parse(path, data)
	if err != nil {
		return f, err
	}
	f.src = src
	errs := f.read(path, s, at, read)
	if f.Compiled != nil {
		f.Warnings = src.warnings(f.Compiled.Warnings())
	}
	return f, src.report(errs)
}

func (f *WorkflowFile) Position(path ...string) (int, int) {
	if f.src == nil {
		return 1, 1
	}
	return f.src.position(path, true)
}

func (f *WorkflowFile) read(path string, s *store.Store, at *store.Entry, read ReadFunc) []error {
	_, invalid := schema.Validate(f.Source)
	var err error
	if f.Doc, err = manifest.Unmarshal(f.Source); err != nil {
		f.Doc = nil
		if len(invalid) == 0 {
			invalid = []error{err}
		}
	}
	if len(invalid) > 0 {
		return invalid
	}
	refs, sources, unresolved := bundle(f.Doc, base(path, s), s, read)
	f.Refs, f.Sources = refs, sources
	w, errs := compile(f.Doc)
	if errs = append(unresolved, errs...); len(errs) > 0 {
		return errs
	}
	f.Compiled = w
	if err := at.CheckName(f.Doc.Name); err != nil {
		errs = append(errs, onValue("name", err))
	}
	if err := at.CheckVersion(f.Doc.Version); err != nil {
		errs = append(errs, onValue("version", err))
	}
	if len(errs) > 0 {
		return errs
	}
	if err := seal.CheckWorkflow(f.Doc); err != nil {
		return []error{onValue("sha", err)}
	}
	return nil
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
