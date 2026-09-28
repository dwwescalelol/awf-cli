package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func bundleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle (<id>[@<version>] | -f <path>)",
		Short: "Inline a workflow's $ref tasks into one document",
		Long: `Inline a workflow's $ref tasks into one document.

A $ref is a path to a task file or a stored task pinned as <id>@<version>.
A relative path resolves against the workflow's directory. An id resolves in
the store in scope. The bundled workflow prints to stdout, or to the file
given by -o, which must lie outside the store.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			out, _ := cmd.Flags().GetString("out")

			data, err := bundle(args, file, out, global)
			if err != nil {
				return err
			}
			if out == "" {
				_, err = os.Stdout.Write(data)
				return err
			}
			return os.WriteFile(out, data, 0o644)
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a workflow file")
	cmd.Flags().StringP("out", "o", "", "write the bundled workflow to this path, outside the store")
	return cmd
}

var (
	errOutIsInput = errors.New("is the workflow being bundled")
	errOutInStore = errors.New("inside the store, whose documents are never overwritten")
)

func bundle(args []string, file, out string, global bool) ([]byte, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return nil, err
	}
	path, _, err := target(scope, store.Workflow, args, file)
	if err != nil {
		return nil, err
	}
	if out != "" {
		if err := writable(out, path, scope); err != nil {
			return nil, err
		}
	}
	doc, err := load.Document(path, scope)
	if err != nil {
		return nil, err
	}
	return manifest.Marshal(doc)
}

// writable refuses an -o path that would overwrite the input or a stored document.
func writable(out, in string, scope *store.Store) error {
	if same(out, in) {
		return fmt.Errorf("-o %s: %w", out, errOutIsInput)
	}
	dir, err := resolved(scope.Dir())
	if err != nil {
		return err
	}
	abs, err := resolved(out)
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(dir, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("-o %s: %s: %w", out, scope.Dir(), errOutInStore)
	}
	return nil
}

func same(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}

// resolved returns path as an absolute path with symlinks in its directory
// resolved, so it compares with the store's resolved directory.
func resolved(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return abs, nil
	}
	return filepath.Join(dir, filepath.Base(abs)), nil
}
