package main

import (
	"fmt"
	"os"

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
			for _, w := range warnings {
				fmt.Fprintln(os.Stderr, w)
			}
			return nil
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "validate a task by id")
	return cmd
}

func formatValid(path string) string { return path + "\nvalid" }

func validate(args []string, file string, task, global bool) (string, []error, error) {
	d, err := resolve(args, file, task, global)
	if err != nil {
		return "", nil, err
	}
	if d.kind == store.Task {
		_, err := load.ReadTask(d.path, d.at)
		return d.path, nil, err
	}
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err != nil {
		return d.path, nil, err
	}
	return d.path, f.Warnings, nil
}
