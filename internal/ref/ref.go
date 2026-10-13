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
	ErrRemote   = errors.New("remote $ref unsupported, install the task and reference it as <namespace>#<id>@<version>")
	ErrUnpinned = errors.New("no version, reference a stored task as [<namespace>#]<id>@<version>")
	ErrNoStore  = errors.New("no store to resolve an id in")
)

type Ref struct {
	Path    string
	ID      store.ID
	Version version.Version
}

func Parse(s string) (Ref, error) {
	if strings.Contains(s, "://") {
		return Ref{}, fmt.Errorf("%q: %w", s, ErrRemote)
	}
	kind, _ := store.KindOf(s)
	if !strings.Contains(s, "#") && (strings.ContainsAny(s, `/\`) || kind == store.Task) {
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

func (r Ref) Locate(dir string, s *store.Store) (string, error) {
	switch {
	case r.Path == "" && s == nil:
		return "", fmt.Errorf("%s@%s: %w", r.ID, r.Version, ErrNoStore)
	case r.Path == "":
		return s.Find(store.Task, r.ID, r.Version)
	case filepath.IsAbs(r.Path):
		return r.Path, nil
	}
	return filepath.Join(dir, r.Path), nil
}
