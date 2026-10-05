package main

import (
	"errors"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func target(scope *store.Store, kind store.DocumentKind, args []string, file string) (string, *store.Entry, error) {
	if file == "" && len(args) == 0 {
		return "", nil, errors.New("give an id or -f")
	}
	if file != "" && len(args) > 0 {
		return "", nil, errors.New("give an id or -f, not both")
	}
	if file != "" {
		return file, nil, nil
	}
	e, err := locate(scope, kind, args[0])
	if err != nil {
		return "", nil, err
	}
	return e.Path, e, nil
}

func locate(scope *store.Store, kind store.DocumentKind, ref string) (*store.Entry, error) {
	name, pin, pinned := strings.Cut(ref, "@")
	id, err := store.NewID(name)
	if err != nil {
		return nil, err
	}
	var v version.Version
	if pinned {
		v, err = version.Parse(pin)
	} else {
		v, err = scope.Latest(kind, id)
	}
	if err != nil {
		return nil, err
	}
	path, err := scope.Find(kind, id, v)
	if err != nil {
		return nil, err
	}
	return &store.Entry{Path: path, ID: id, Version: v}, nil
}
