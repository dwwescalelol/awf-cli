package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
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

func validate(args []string, file string, task, global bool) (string, []error, error) {
	kind := store.KindOf(file)
	if task {
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
		return path, nil, at.Check("", t.Version)
	}
	w, err := load.Workflow(path, scope)
	if err != nil {
		return path, nil, err
	}
	return path, w.Warnings(), at.Check(w.Name, w.Version)
}
