package load

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func bundle(doc *manifest.Workflow, dir string, s *store.Store) ([]string, error) {
	var paths []string
	var errs []error
	for _, name := range sorted(doc.Tasks) {
		entry := doc.Tasks[name]
		if entry.Task != nil {
			continue
		}
		t, path, err := resolve(entry.Ref, dir, s)
		if path != "" {
			paths = append(paths, path)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("tasks/%s: %w %q: %w", name, ErrUnresolvedRef, entry.Ref, err))
			continue
		}
		doc.Tasks[name] = manifest.TaskEntry{Task: t}
	}
	return paths, errors.Join(errs...)
}

func resolve(raw, dir string, s *store.Store) (*manifest.Task, string, error) {
	r, err := ref.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	path, err := r.Locate(dir, s)
	if err != nil {
		return nil, "", err
	}
	t, err := Task(path)
	if err != nil {
		return nil, path, err
	}
	if r.Path == "" {
		if err := (&store.Entry{Path: path, ID: r.ID, Version: r.Version}).Check("", t.Version); err != nil {
			return nil, path, err
		}
	}
	t.OpenAWF = version.Version{}
	return t, path, nil
}
