package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

// locate turns a command argument into a file. id@version names an installed
// document, and a bare id its highest version. An argument that is a file on
// disk is that file, so a path always beats an id.
func locate(arg string, global bool) (string, error) {
	id, version, pinned := strings.Cut(arg, "@")
	if !pinned {
		if _, err := os.Stat(arg); err == nil {
			return arg, nil
		}
	}
	if pinned && (id == "" || version == "") {
		return "", fmt.Errorf("%q: not a path or id@version", arg)
	}

	scope, err := store.Resolve(global)
	if err != nil {
		return "", err
	}
	if !pinned {
		if version, err = scope.Latest(store.Workflow, id); err != nil {
			return "", err
		}
	}
	return scope.Path(store.Workflow, id, version), nil
}
