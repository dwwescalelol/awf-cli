package load

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

var ErrUnresolvedRef = errors.New("unresolved $ref")

// Document reads a workflow and inlines every $ref task into tasks. It reports
// every $ref that fails to resolve at once.
func Document(path string, s *store.Store) (*manifest.Workflow, error) {
	doc, err := read(path)
	if err != nil {
		return nil, err
	}
	if err := bundle(doc, filepath.Dir(path), s); err != nil {
		return nil, err
	}
	return doc, nil
}

// bundle replaces each $ref entry with the task it names. An entry that fails
// to resolve stays a $ref.
func bundle(doc *manifest.Workflow, dir string, s *store.Store) error {
	var errs []error
	for _, name := range sorted(doc.Tasks) {
		entry := doc.Tasks[name]
		if entry.Task != nil {
			continue
		}
		t, err := resolve(entry.Ref, dir, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("tasks/%s %q: %w: %w", name, entry.Ref, ErrUnresolvedRef, err))
			continue
		}
		doc.Tasks[name] = manifest.TaskEntry{Task: t}
	}
	return errors.Join(errs...)
}

func resolve(raw, dir string, s *store.Store) (*manifest.Task, error) {
	r, err := ref.Parse(raw)
	if err != nil {
		return nil, err
	}
	path, err := r.Locate(dir, s)
	if err != nil {
		return nil, err
	}
	t, err := Task(path)
	if err != nil {
		return nil, err
	}
	// The workflow declares openawf for the tasks it holds.
	t.OpenAWF = version.Version{}
	return t, nil
}
