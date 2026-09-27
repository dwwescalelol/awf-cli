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
	cmd.Flags().Bool("global", false, "act on ~/.awf")
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

func validate(args []string, file string, task, global bool) (string, []error, error) {
	kind := store.Workflow
	if task || strings.HasSuffix(file, ".md") {
		kind = store.Task
	}
	path, err := target(kind, args, file, global)
	if err != nil {
		return "", nil, err
	}
	if kind == store.Task {
		_, err = load.Task(path)
		return path, nil, err
	}
	w, err := load.Workflow(path)
	if err != nil {
		return path, nil, err
	}
	return path, w.Warnings(), nil
}

func target(kind store.DocumentKind, args []string, file string, global bool) (string, error) {
	if file == "" && len(args) == 0 {
		return "", errors.New("give an id or -f")
	}
	if file != "" && len(args) > 0 {
		return "", errors.New("give an id or -f, not both")
	}
	if file != "" {
		return file, nil
	}

	scope, err := store.Resolve(global)
	if err != nil {
		return "", err
	}
	return locate(scope, kind, args[0])
}

func locate(scope *store.Store, kind store.DocumentKind, ref string) (string, error) {
	name, pin, pinned := strings.Cut(ref, "@")
	id, err := store.NewID(name)
	if err != nil {
		return "", err
	}
	if pinned {
		v, err := version.Parse(pin)
		if err != nil {
			return "", err
		}
		return scope.Find(kind, id, v)
	}

	v, err := scope.Latest(kind, id)
	if err != nil {
		return "", err
	}
	return scope.Find(kind, id, v)
}
