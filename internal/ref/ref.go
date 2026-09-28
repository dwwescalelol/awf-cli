// Package ref parses the $ref of a task entry and locates the task file it names.
package ref

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

var (
	ErrRemote   = errors.New("remote $ref unsupported, install the task and reference it as <id>@<version>")
	ErrUnpinned = errors.New("no version, reference a stored task as <id>@<version>")
)

// Ref names a task file by path, or a stored task by id and exact version.
type Ref struct {
	Path    string
	ID      store.ID
	Version version.Version
}

// Parse reads a $ref. A value holding a path separator or ending in .md is a
// path. Anything else is <id>@<version>.
func Parse(s string) (Ref, error) {
	if strings.Contains(s, "://") {
		return Ref{}, fmt.Errorf("%q: %w", s, ErrRemote)
	}
	if strings.ContainsAny(s, `/\`) || strings.HasSuffix(s, ".md") {
		return Ref{Path: s}, nil
	}
	name, pin, pinned := strings.Cut(s, "@")
	if !pinned {
		return Ref{}, fmt.Errorf("%q: %w", s, ErrUnpinned)
	}
	id, err := store.NewID(name)
	if err != nil {
		return Ref{}, err
	}
	v, err := version.Parse(pin)
	if err != nil {
		return Ref{}, err
	}
	return Ref{ID: id, Version: v}, nil
}

// Locate returns the file r names. A relative path resolves against dir, the
// directory of the document holding the ref. An id resolves in s.
func (r Ref) Locate(dir string, s *store.Store) (string, error) {
	switch {
	case r.Path == "":
		return s.Find(store.Task, r.ID, r.Version)
	case filepath.IsAbs(r.Path):
		return r.Path, nil
	}
	return filepath.Join(dir, r.Path), nil
}
