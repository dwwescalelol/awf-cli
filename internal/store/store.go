// Package store locates the .awf directories a command reads and writes.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

const dirName = ".awf"

// Kind is a document tree inside a scope. The value is the directory name.
type Kind string

const (
	Workflow Kind = "wf"
	Task     Kind = "task"
)

func (k Kind) String() string {
	switch k {
	case Workflow:
		return "workflow"
	case Task:
		return "task"
	}
	return string(k)
}

func (k Kind) ext() string {
	switch k {
	case Workflow:
		return ".yaml"
	case Task:
		return ".md"
	}
	return ""
}

// Scope is one .awf directory.
type Scope struct {
	Dir string
}

// Resolve returns the scope to act on.
func Resolve(global bool) (Scope, error) {
	if global {
		return globalScope()
	}
	dir, err := os.Getwd()
	if err != nil {
		return Scope{}, err
	}
	for {
		if isDir(filepath.Join(dir, dirName)) {
			return Scope{Dir: filepath.Join(dir, dirName)}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return globalScope()
		}
		dir = parent
	}
}

func globalScope() (Scope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Scope{}, err
	}
	return Scope{Dir: filepath.Join(home, dirName)}, nil
}

func (s Scope) Path(kind Kind, id string, v version.Version) string {
	return filepath.Join(s.Dir, string(kind), id, v.String()+kind.ext())
}

// Mkdir makes the directory Path writes into. Scopes appear on first write.
func (s Scope) Mkdir(kind Kind, id string) error {
	return os.MkdirAll(filepath.Join(s.Dir, string(kind), id), 0o755)
}

// Entry is an id and the versions stored under it.
type Entry struct {
	ID       string
	Versions []version.Version
}

func (s Scope) List(kind Kind) ([]Entry, error) {
	ids, err := os.ReadDir(filepath.Join(s.Dir, string(kind)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// os.ReadDir sorts by filename, so both ids and versions come out in a
	// stable order.
	var entries []Entry
	for _, id := range ids {
		if !id.IsDir() {
			continue
		}
		versions, err := s.versions(kind, id.Name())
		if err != nil {
			return nil, err
		}
		if len(versions) > 0 {
			entries = append(entries, Entry{ID: id.Name(), Versions: versions})
		}
	}
	return entries, nil
}

func (s Scope) versions(kind Kind, id string) ([]version.Version, error) {
	files, err := os.ReadDir(filepath.Join(s.Dir, string(kind), id))
	if err != nil {
		return nil, err
	}
	var versions []version.Version
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || !strings.HasSuffix(name, kind.ext()) {
			continue
		}
		v, err := version.Parse(strings.TrimSuffix(name, kind.ext()))
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// Latest is the highest version stored under an id. Versions sort numerically,
// so 0.10.0 beats 0.9.0.
func (s Scope) Latest(kind Kind, id string) (version.Version, error) {
	versions, err := s.versions(kind, id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return version.Version{}, err
	}
	if len(versions) == 0 {
		return version.Version{}, fmt.Errorf("%s %q: not installed in %s", kind, id, s.Dir)
	}

	return slices.MaxFunc(versions, version.Version.Compare), nil
}

// Locate turns a reference into a file. id@version names a stored document,
// and a bare id its highest version. A reference that is a file on disk is
// that file, so a path always beats an id.
func (s Scope) Locate(kind Kind, ref string) (string, error) {
	id, pin, pinned := strings.Cut(ref, "@")
	if !pinned {
		if _, err := os.Stat(ref); err == nil {
			return ref, nil
		}
	}
	if pinned && (id == "" || pin == "") {
		return "", fmt.Errorf("%q: not a path or id@version", ref)
	}

	v, err := s.Latest(kind, id)
	if pinned {
		v, err = version.Parse(pin)
	}
	if err != nil {
		return "", err
	}
	return s.Path(kind, id, v), nil
}
