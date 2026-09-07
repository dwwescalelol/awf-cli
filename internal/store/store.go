// Package store locates the .awf directories a command reads and writes.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

const dirName = ".awf"

type DocumentKind struct {
	dir  string
	ext  string
	name string
}

var (
	Workflow = DocumentKind{dir: "wf", ext: ".yaml", name: "workflow"}
	Task     = DocumentKind{dir: "task", ext: ".md", name: "task"}
)

func (k DocumentKind) String() string { return k.name }

type ID string

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func NewID(s string) (ID, error) {
	if !idPattern.MatchString(s) {
		return "", fmt.Errorf("%q: %w", s, ErrNotAnID)
	}
	return ID(s), nil
}

func (id ID) String() string { return string(id) }

// Scope represents the structure of a .awf directory
type Scope struct {
	Dir string
}

// Finds the closest ansestor .awf dir from the current working dir.
// If global is true, returns the homes ~/.awf dir.
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

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func globalScope() (Scope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Scope{}, err
	}
	return Scope{Dir: filepath.Join(home, dirName)}, nil
}

func (s Scope) kindDir(kind DocumentKind) string {
	return filepath.Join(s.Dir, kind.dir)
}

func (s Scope) docDir(kind DocumentKind, id ID) string {
	return filepath.Join(s.kindDir(kind), string(id))
}

func (s Scope) Path(kind DocumentKind, id ID, v version.Version) string {
	return filepath.Join(s.docDir(kind, id), v.String()+kind.ext)
}

// Mkdir makes the directory Path writes into. Scopes appear on first write.
func (s Scope) Mkdir(kind DocumentKind, id ID) error {
	return os.MkdirAll(s.docDir(kind, id), 0o755)
}

var ErrNotInstalled = errors.New("not installed")

func (s Scope) Latest(kind DocumentKind, id ID) (version.Version, error) {
	versions, _, err := s.versions(kind, id)
	if err != nil {
		return version.Version{}, err
	}
	if len(versions) == 0 {
		return version.Version{}, fmt.Errorf("%s %q in %s: %w", kind, id, s.Dir, ErrNotInstalled)
	}

	return slices.MaxFunc(versions, version.Version.Compare), nil
}

func (s Scope) Find(kind DocumentKind, id ID, v version.Version) (string, error) {
	path := s.Path(kind, id, v)
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		return "", fmt.Errorf("%s %q version %s in %s: %w", kind, id, v, s.Dir, ErrNotInstalled)
	}
	return path, nil
}
