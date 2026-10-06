package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

const (
	EnvHome  = "AWF_HOME"
	dirName  = ".awf"
	dirPerm  = 0o755
	filePerm = 0o644
)

var (
	ErrNotAnID      = errors.New("not an id")
	ErrNotADocument = errors.New("not a document file")
	ErrNestedDir    = errors.New("unexpected directory")
	ErrNotInstalled = errors.New("not installed")
	ErrExists       = errors.New("already exists")
	ErrWrongName    = errors.New("name does not match the id it is stored under")
	ErrWrongVersion = errors.New("version does not match the version it is stored under")
)

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

func KindOf(path string) (DocumentKind, error) {
	switch ext := filepath.Ext(path); {
	case strings.EqualFold(ext, Task.ext):
		return Task, nil
	case strings.EqualFold(ext, Workflow.ext):
		return Workflow, nil
	}
	return DocumentKind{}, fmt.Errorf("%s: %w", path, ErrNotADocument)
}

type ID string

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func NewID(s string) (ID, error) {
	if !idPattern.MatchString(s) {
		return "", fmt.Errorf("%q: %w", s, ErrNotAnID)
	}
	return ID(s), nil
}

func (id ID) String() string { return string(id) }

type Document struct {
	ID       ID
	Versions []version.Version
}

type Skipped struct {
	Path   string
	Reason error
}

func (sk Skipped) String() string { return fmt.Sprintf("%s: %v", sk.Path, sk.Reason) }

func GlobalDir() (string, error) {
	if dir := os.Getenv(EnvHome); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName), nil
}

func FindDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	for {
		if dir == home {
			return GlobalDir()
		}
		candidate := filepath.Join(dir, dirName)
		if isDir(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return GlobalDir()
		}
		dir = parent
	}
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

type Store struct {
	dir string
}

func New(dir string) *Store { return &Store{dir: dir} }

type Entry struct {
	Path    string
	ID      ID
	Version version.Version
}

func (e *Entry) CheckName(name string) error {
	if e == nil || name == e.ID.String() {
		return nil
	}
	return fmt.Errorf("name %q, stored as %q: %w", name, e.ID, ErrWrongName)
}

func (e *Entry) CheckVersion(v version.Version) error {
	if e == nil || v.Compare(e.Version) == 0 {
		return nil
	}
	return fmt.Errorf("version %s, stored as %s: %w", v, e.Version, ErrWrongVersion)
}

func Resolve(global bool) (*Store, error) {
	locate := FindDir
	if global {
		locate = GlobalDir
	}
	dir, err := locate()
	if err != nil {
		return nil, err
	}
	return New(dir), nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) kindDir(kind DocumentKind) string {
	return filepath.Join(s.dir, kind.dir)
}

func (s *Store) docDir(kind DocumentKind, id ID) string {
	return filepath.Join(s.kindDir(kind), string(id))
}

func (s *Store) Path(kind DocumentKind, id ID, v version.Version) string {
	return filepath.Join(s.docDir(kind, id), v.String()+kind.ext)
}

func (s *Store) Find(kind DocumentKind, id ID, v version.Version) (string, error) {
	path := s.Path(kind, id, v)
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		return path, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return "", fmt.Errorf("%s %q version %s in %s: %w", kind, id, v, s.dir, ErrNotInstalled)
}

func (s *Store) Latest(kind DocumentKind, id ID) (version.Version, error) {
	versions, _, err := s.versions(kind, id)
	if err != nil {
		return version.Version{}, err
	}
	if len(versions) == 0 {
		return version.Version{}, fmt.Errorf("%s %q in %s: %w", kind, id, s.dir, ErrNotInstalled)
	}
	return slices.MaxFunc(versions, version.Version.Compare), nil
}

func (s *Store) Read(kind DocumentKind, id ID, v version.Version) ([]byte, error) {
	path, err := s.Find(kind, id, v)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (s *Store) Write(kind DocumentKind, id ID, v version.Version, data []byte) error {
	if err := os.MkdirAll(s.docDir(kind, id), dirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path(kind, id, v), os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s %q version %s in %s: %w", kind, id, v, s.dir, ErrExists)
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *Store) List(kind DocumentKind) ([]Document, []Skipped, error) {
	root := s.kindDir(kind)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var documents []Document
	var skipped []Skipped
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(root, name)
		if !entry.IsDir() {
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNotADocument})
			continue
		}
		id, err := NewID(name)
		if err != nil {
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNotAnID})
			continue
		}
		versions, versionSkipped, err := s.versions(kind, id)
		if err != nil {
			return nil, nil, err
		}
		skipped = append(skipped, versionSkipped...)
		if len(versions) > 0 {
			documents = append(documents, Document{ID: id, Versions: versions})
		}
	}
	return documents, skipped, nil
}

func (s *Store) versions(kind DocumentKind, id ID) ([]version.Version, []Skipped, error) {
	root := s.docDir(kind, id)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var versions []version.Version
	var skipped []Skipped
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(root, name)
		switch {
		case entry.IsDir():
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNestedDir})
		case !strings.HasSuffix(name, kind.ext):
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNotADocument})
		default:
			v, err := version.Parse(strings.TrimSuffix(name, kind.ext))
			if err != nil {
				skipped = append(skipped, Skipped{Path: path, Reason: err})
				continue
			}
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, version.Version.Compare)
	return versions, skipped, nil
}
