package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	run := func(_ *cobra.Command, r request) error {
		path, warnings, err := validate(r)
		if err != nil {
			return err
		}
		fmt.Println(formatValid(path))
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
		}
		return nil
	}
	return kindCmd(&cobra.Command{
		Use:   "validate",
		Short: "Check a workflow or task document against the OpenAWF spec",
	},
		documentCmd(store.Workflow, "Check a workflow against the OpenAWF spec", run),
		documentCmd(store.Task, "Check a task against the OpenAWF spec", run),
	)
}

func formatValid(path string) string { return path + "\nvalid" }

func validate(r request) (string, load.Problems, error) {
	d, err := resolve(r)
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
