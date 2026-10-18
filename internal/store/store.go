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
	Local    = "_"
	Draft    = "draft"
	dirName  = ".awf"
	dirPerm  = 0o755
	filePerm = 0o644
)

var (
	ErrNotAnID       = errors.New("not an id")
	ErrNotANamespace = errors.New("not a namespace")
	ErrNotADocument  = errors.New("not a document file")
	ErrNestedDir     = errors.New("unexpected directory")
	ErrNotInstalled  = errors.New("not installed")
	ErrExists        = errors.New("already exists")
	ErrWrongName     = errors.New("name does not match the id it is stored under")
	ErrWrongVersion  = errors.New("version does not match the version it is stored under")
	ErrInStore       = errors.New("already inside a project store")
	ErrReadOnly      = errors.New("read-only namespace")
	ErrNoDraft       = errors.New("no draft")
	ErrFileName      = errors.New("a document in a scope is named <major.minor.patch> or " + Draft)
)

type DocumentKind struct {
	dir  string
	ext  string
	name string
}

var (
	Workflow = DocumentKind{dir: "workflow", ext: ".yaml", name: "workflow"}
	Task     = DocumentKind{dir: "task", ext: ".md", name: "task"}
)

func (k DocumentKind) String() string { return k.name }

func (k DocumentKind) Dir() string { return k.dir }

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

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	segmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

func NewID(s string) (ID, error) {
	name := s
	if namespace, after, qualified := strings.Cut(s, "#"); qualified {
		if !validNamespace(namespace) {
			return "", fmt.Errorf("%q: %w", s, ErrNotANamespace)
		}
		name = after
	}
	if !idPattern.MatchString(name) {
		return "", fmt.Errorf("%q: %w", s, ErrNotAnID)
	}
	return ID(s), nil
}

func newIDIn(namespace, name string) (ID, error) {
	if namespace != Local {
		name = namespace + "#" + name
	}
	return NewID(name)
}

func validNamespace(s string) bool {
	for segment := range strings.SplitSeq(s, "/") {
		if !segmentPattern.MatchString(segment) || segment == Workflow.dir || segment == Task.dir {
			return false
		}
	}
	return true
}

func (id ID) String() string { return string(id) }

func (id ID) CheckLocal() error {
	if id.Namespace() != Local {
		return fmt.Errorf("%q: %w, only %s is writable", id, ErrReadOnly, Local)
	}
	return nil
}

func (id ID) Namespace() string {
	if namespace, _, qualified := strings.Cut(string(id), "#"); qualified {
		return namespace
	}
	return Local
}

func (id ID) Name() string {
	if _, name, qualified := strings.Cut(string(id), "#"); qualified {
		return name
	}
	return string(id)
}

// CheckFileName rejects a document in a scope named other than a version or
// draft. It reports whether path is a version in a scope.
func CheckFileName(path string) (bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, nil
	}
	dir := filepath.Dir(abs)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	root, err := FindDirFrom(dir)
	if err != nil {
		return false, nil
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	name := filepath.Base(abs)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == Draft {
		return false, nil
	}
	if _, err := version.Parse(stem); err != nil {
		return false, fmt.Errorf("%s: %w", name, ErrFileName)
	}
	return true, nil
}

type Document struct {
	ID       ID
	Versions []version.Version
	Draft    bool
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
	return FindDirFrom(dir)
}

func FindDirFrom(dir string) (string, error) {
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
	Draft   bool
}

func (e *Entry) CheckName(name string) error {
	if e == nil || name == e.ID.Name() {
		return nil
	}
	return fmt.Errorf("name %q, stored as %q: %w", name, e.ID.Name(), ErrWrongName)
}

func (e *Entry) CheckVersion(v version.Version) error {
	if e == nil || e.Draft || v.Compare(e.Version) == 0 {
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

func Init(dir string) (*Store, error) {
	found, err := FindDirFrom(dir)
	if err != nil {
		return nil, err
	}
	global, err := GlobalDir()
	if err != nil {
		return nil, err
	}
	if found != global {
		return nil, fmt.Errorf("%s: %w", found, ErrInStore)
	}
	s := New(filepath.Join(dir, dirName))
	for _, kind := range []DocumentKind{Workflow, Task} {
		if err := os.MkdirAll(s.kindDir(Local, kind), dirPerm); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) kindDir(namespace string, kind DocumentKind) string {
	return filepath.Join(s.dir, filepath.FromSlash(namespace), kind.dir)
}

func (s *Store) docDir(kind DocumentKind, id ID) string {
	return filepath.Join(s.kindDir(id.Namespace(), kind), id.Name())
}

func (s *Store) Path(kind DocumentKind, id ID, v version.Version) string {
	return filepath.Join(s.docDir(kind, id), v.String()+kind.ext)
}

func (s *Store) DraftPath(kind DocumentKind, id ID) string {
	return filepath.Join(s.docDir(kind, id), Draft+kind.ext)
}

func (s *Store) FindDraft(kind DocumentKind, id ID) (string, error) {
	path := s.DraftPath(kind, id)
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		return path, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return "", fmt.Errorf("%s %q in %s: %w", kind, id, s.dir, ErrNoDraft)
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
	err := s.create(s.Path(kind, id, v), data)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s %q version %s in %s: %w", kind, id, v, s.dir, ErrExists)
	}
	return err
}

func (s *Store) WriteDraft(kind DocumentKind, id ID, data []byte) error {
	err := s.create(s.DraftPath(kind, id), data)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s %q draft in %s: %w", kind, id, s.dir, ErrExists)
	}
	return err
}

func (s *Store) Seal(kind DocumentKind, id ID, v version.Version, data []byte) error {
	if err := s.Write(kind, id, v, data); err != nil {
		return err
	}
	err := os.Remove(s.DraftPath(kind, id))
	if err != nil {
		os.Remove(s.Path(kind, id, v))
	}
	return err
}

func (s *Store) EntryAt(path string) (*Entry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := s.dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(resolved, filepath.Base(abs))
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, nil
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 4 {
		return nil, nil
	}
	file, name, kindDir := parts[len(parts)-1], parts[len(parts)-2], parts[len(parts)-3]
	namespace := strings.Join(parts[:len(parts)-3], "/")
	kind, err := KindOf(file)
	if err != nil || kind.dir != kindDir {
		return nil, nil
	}
	id, err := newIDIn(namespace, name)
	if err != nil {
		return nil, nil
	}
	e := &Entry{Path: path, ID: id}
	stem := strings.TrimSuffix(file, filepath.Ext(file))
	if strings.EqualFold(stem, Draft) {
		e.Draft = true
		return e, nil
	}
	if e.Version, err = version.Parse(stem); err != nil {
		return nil, nil
	}
	return e, nil
}

func (s *Store) create(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *Store) Replace(kind DocumentKind, id ID, v version.Version, data []byte) error {
	path, err := s.Find(kind, id, v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, filePerm)
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
	var documents []Document
	var skipped []Skipped
	err := filepath.WalkDir(s.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == s.dir {
			return err
		}
		switch entry.Name() {
		case kind.dir:
		case Workflow.dir, Task.dir:
			return filepath.SkipDir
		default:
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(s.dir, filepath.Dir(path))
		if err != nil {
			return err
		}
		namespace := filepath.ToSlash(rel)
		if namespace != Local && !validNamespace(namespace) {
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNotANamespace})
			return filepath.SkipDir
		}
		found, foundSkipped, err := s.listIn(namespace, kind)
		if err != nil {
			return err
		}
		documents = append(documents, found...)
		skipped = append(skipped, foundSkipped...)
		return filepath.SkipDir
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return documents, skipped, nil
}

func (s *Store) listIn(namespace string, kind DocumentKind) ([]Document, []Skipped, error) {
	root := s.kindDir(namespace, kind)
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
		id, err := newIDIn(namespace, name)
		if err != nil {
			skipped = append(skipped, Skipped{Path: path, Reason: ErrNotAnID})
			continue
		}
		versions, versionSkipped, err := s.versions(kind, id)
		if err != nil {
			return nil, nil, err
		}
		skipped = append(skipped, versionSkipped...)
		_, draftErr := s.FindDraft(kind, id)
		draft := draftErr == nil
		if len(versions) > 0 || draft {
			documents = append(documents, Document{ID: id, Versions: versions, Draft: draft})
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
		if strings.HasPrefix(name, ".") || strings.EqualFold(name, Draft+kind.ext) && !entry.IsDir() {
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
