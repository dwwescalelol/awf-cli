package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestInit(t *testing.T) {
	root := t.TempDir()
	t.Setenv(store.EnvHome, filepath.Join(root, "home"))
	t.Chdir(root)

	s, err := initStore()
	if err != nil {
		t.Fatal(err)
	}
	if s.Dir() != filepath.Join(root, ".awf") {
		t.Errorf("got %s, want %s", s.Dir(), filepath.Join(root, ".awf"))
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), store.Local, store.Workflow.Dir())); err != nil {
		t.Error(err)
	}

	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "sub"))
	if _, err := initStore(); !errors.Is(err, store.ErrInStore) {
		t.Errorf("got %v, want %v", err, store.ErrInStore)
	}
}
