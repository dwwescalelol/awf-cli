package load

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

var ErrUnresolvedRef = errors.New("unresolved $ref")

func bundle(doc *manifest.Workflow, dir string, s *store.Store) ([]string, []error) {
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
			errs = append(errs, unresolved(name, entry.Ref, err)...)
			continue
		}
		doc.Tasks[name] = manifest.TaskEntry{Task: t}
	}
	return paths, errs
}

func unresolved(name, ref string, err error) []error {
	wrap := func(err error) error {
		return fmt.Errorf("tasks/%s: %w %q: %w", name, ErrUnresolvedRef, ref, err)
	}
	var problems Problems
	if !errors.As(err, &problems) {
		return []error{onValue("tasks/"+name+"/$ref", wrap(err))}
	}
	out := make([]error, 0, len(problems))
	for _, p := range problems {
		out = append(out, &Problem{File: p.File, Line: p.Line, Column: p.Column, Err: wrap(p.Err)})
	}
	return out
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
	var at *store.Entry
	if r.Path == "" {
		at = &store.Entry{Path: path, ID: r.ID, Version: r.Version}
	}
	f, err := ReadTask(path, at)
	if err != nil {
		return nil, path, err
	}
	t := f.Doc
	t.OpenAWF = version.Version{}
	return t, path, nil
}
