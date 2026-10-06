package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

type document struct {
	kind  store.DocumentKind
	scope *store.Store
	path  string
	at    *store.Entry
}

func resolve(args []string, file string, task, global bool, accept ...store.DocumentKind) (document, error) {
	d := document{kind: store.Workflow, path: file}
	var err error
	if file != "" {
		if d.kind, err = store.KindOf(file); err != nil {
			return document{}, err
		}
	}
	if task {
		d.kind = store.Task
	}
	if len(accept) > 0 && !slices.Contains(accept, d.kind) {
		return document{}, fmt.Errorf("%s: a %s, want a %s: %w", file, d.kind, accept[0], errWrongKind)
	}
	if d.scope, err = store.Resolve(global); err != nil {
		return document{}, err
	}
	if d.at, err = target(d.scope, d.kind, args, file); err != nil {
		return document{}, err
	}
	if d.at != nil {
		d.path = d.at.Path
	}
	return d, nil
}

var errWrongKind = errors.New("wrong document kind")

func target(scope *store.Store, kind store.DocumentKind, args []string, file string) (*store.Entry, error) {
	if file == "" && len(args) == 0 {
		return nil, errors.New("give an id or -f")
	}
	if file != "" && len(args) > 0 {
		return nil, errors.New("give an id or -f, not both")
	}
	if file != "" {
		return nil, nil
	}
	return locate(scope, kind, args[0])
}

func parseTarget(arg string) (store.ID, version.Version, bool, error) {
	name, pin, pinned := strings.Cut(arg, "@")
	id, err := store.NewID(name)
	if err != nil {
		return "", version.Version{}, false, err
	}
	if !pinned {
		return id, version.Version{}, false, nil
	}
	v, err := version.Parse(pin)
	if err != nil {
		return "", version.Version{}, false, err
	}
	return id, v, true, nil
}

func locate(scope *store.Store, kind store.DocumentKind, arg string) (*store.Entry, error) {
	id, v, pinned, err := parseTarget(arg)
	if err != nil {
		return nil, err
	}
	if !pinned {
		if v, err = scope.Latest(kind, id); err != nil {
			return nil, err
		}
	}
	path, err := scope.Find(kind, id, v)
	if err != nil {
		return nil, err
	}
	return &store.Entry{Path: path, ID: id, Version: v}, nil
}
