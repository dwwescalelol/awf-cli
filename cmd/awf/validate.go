package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate ([-t] <id>[@<version>] | -f <path>)",
		Short: "Check a workflow or task document against the OpenAWF spec",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetBool("task")

			path, warnings, err := validate(args, file, task, global)
			if err != nil {
				return err
			}
			fmt.Println(formatValid(path))
			if len(warnings) > 0 {
				fmt.Fprintln(os.Stderr, formatWarnings(warnings))
			}
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "validate a task by id")
	return cmd
}

func formatValid(path string) string { return path + "\nvalid" }

func formatErr(err error) string { return err.Error() }

func formatWarnings(warnings []error) string {
	lines := make([]string, 0, len(warnings))
	for _, w := range warnings {
		lines = append(lines, "warning: "+w.Error())
	}
	return strings.Join(lines, "\n")
}

var (
	errWrongName    = errors.New("name does not match the id it is stored under")
	errWrongVersion = errors.New("version does not match the version it is stored under")
)

type stored struct {
	id      store.ID
	version version.Version
}

func validate(args []string, file string, task, global bool) (string, []error, error) {
	kind := store.Workflow
	if task || strings.HasSuffix(file, ".md") {
		kind = store.Task
	}
	scope, err := store.Resolve(global)
	if err != nil {
		return "", nil, err
	}
	path, at, err := target(scope, kind, args, file)
	if err != nil {
		return "", nil, err
	}
	if kind == store.Task {
		t, err := load.Task(path)
		if err != nil {
			return path, nil, err
		}
		return path, nil, at.check("", t.Version)
	}
	w, err := load.Workflow(path, scope)
	if err != nil {
		return path, nil, err
	}
	return path, w.Warnings(), at.check(w.Name, w.Version)
}

func (at *stored) check(name string, v version.Version) error {
	if at == nil {
		return nil
	}
	var errs []error
	if name != "" && name != at.id.String() {
		errs = append(errs, fmt.Errorf("name %q, stored as %q: %w", name, at.id, errWrongName))
	}
	if v.Compare(at.version) != 0 {
		errs = append(errs, fmt.Errorf("version %s, stored as %s: %w", v, at.version, errWrongVersion))
	}
	return errors.Join(errs...)
}

func target(scope *store.Store, kind store.DocumentKind, args []string, file string) (string, *stored, error) {
	if file == "" && len(args) == 0 {
		return "", nil, errors.New("give an id or -f")
	}
	if file != "" && len(args) > 0 {
		return "", nil, errors.New("give an id or -f, not both")
	}
	if file != "" {
		return file, nil, nil
	}
	return locate(scope, kind, args[0])
}

func locate(scope *store.Store, kind store.DocumentKind, ref string) (string, *stored, error) {
	name, pin, pinned := strings.Cut(ref, "@")
	id, err := store.NewID(name)
	if err != nil {
		return "", nil, err
	}
	var v version.Version
	if pinned {
		v, err = version.Parse(pin)
	} else {
		v, err = scope.Latest(kind, id)
	}
	if err != nil {
		return "", nil, err
	}
	path, err := scope.Find(kind, id, v)
	if err != nil {
		return "", nil, err
	}
	return path, &stored{id: id, version: v}, nil
}
