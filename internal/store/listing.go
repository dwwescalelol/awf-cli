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

var (
	ErrNotAnID      = errors.New("not an id")
	ErrNotADocument = errors.New("not a document file")
	ErrNestedDir    = errors.New("unexpected directory")
)

// Document is an id and the versions stored under it.
type Document struct {
	ID       ID
	Versions []version.Version
}

// Skipped is a path in the store that could not be read as part of a document.
type Skipped struct {
	Path   string
	Reason error
}

func (s Scope) List(kind DocumentKind) ([]Document, []Skipped, error) {
	dirEntries, err := os.ReadDir(s.kindDir(kind))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var documents []Document
	var skipped []Skipped
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		name := dirEntry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		id, err := NewID(name)
		if err != nil {
			skipped = append(skipped, Skipped{
				Path:   filepath.Join(s.kindDir(kind), name),
				Reason: ErrNotAnID,
			})
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

func (s Scope) versions(kind DocumentKind, id ID) ([]version.Version, []Skipped, error) {
	dirEntries, err := os.ReadDir(s.docDir(kind, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var versions []version.Version
	var skipped []Skipped
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(s.docDir(kind, id), name)
		switch {
		case dirEntry.IsDir():
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

func (sk Skipped) String() string {
	return fmt.Sprintf("%s: %v", sk.Path, sk.Reason)
}
