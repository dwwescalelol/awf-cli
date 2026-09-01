// Package store locates the .awf directories a command reads and writes.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (s Scope) Path(kind Kind, id, version string) string {
	return filepath.Join(s.Dir, string(kind), id, version+kind.ext())
}

// Mkdir makes the directory Path writes into. Scopes appear on first write.
func (s Scope) Mkdir(kind Kind, id string) error {
	return os.MkdirAll(filepath.Join(s.Dir, string(kind), id), 0o755)
}

// Entry is an id and the versions stored under it.
type Entry struct {
	ID       string
	Versions []string
}

// List reports what a scope holds. A scope that has never been written to
// holds nothing, which is not an error.
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

func (s Scope) versions(kind Kind, id string) ([]string, error) {
	files, err := os.ReadDir(filepath.Join(s.Dir, string(kind), id))
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || !strings.HasSuffix(name, kind.ext()) {
			continue
		}
		versions = append(versions, strings.TrimSuffix(name, kind.ext()))
	}
	return versions, nil
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// Latest is the highest version stored under an id. Versions sort numerically,
// so 0.10.0 beats 0.9.0.
func (s Scope) Latest(kind Kind, id string) (string, error) {
	versions, err := s.versions(kind, id)
	if errors.Is(err, fs.ErrNotExist) || len(versions) == 0 {
		return "", fmt.Errorf("no %s %q is installed in %s", kind, id, s.Dir)
	}
	if err != nil {
		return "", err
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if compareVersions(v, latest) > 0 {
			latest = v
		}
	}
	return latest, nil
}

// compareVersions orders dotted numbers. A component that is not a number
// sorts below one that is, so a stray filename never wins.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(as), len(bs)); i++ {
		if c := component(as, i) - component(bs, i); c != 0 {
			return c
		}
	}
	return strings.Compare(a, b)
}

func component(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return -1
	}
	return n
}
