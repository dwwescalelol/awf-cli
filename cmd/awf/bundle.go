package main

import (
	"os"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func bundleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle (<id>[@<version>] | -f <path>)",
		Short: "Dereference a workflow's $ref tasks into one document",
		Long: `Dereference a workflow's $ref tasks into one document.

A $ref is a path to a task file or a stored task pinned as <id>@<version>.
A relative path resolves against the directory holding the store in scope.
An id resolves in the store in scope. The bundled workflow prints to stdout,
or to the file given by -o.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			out, _ := cmd.Flags().GetString("out")

			data, err := bundle(args, file, out, global)
			if err != nil {
				return err
			}
			if out != "" {
				return nil
			}
			_, err = os.Stdout.Write(data)
			return err
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a workflow file")
	cmd.Flags().StringP("out", "o", "", "write the bundled workflow to this path")
	return cmd
}

func bundle(args []string, file, out string, global bool) ([]byte, error) {
	d, err := resolve(args, file, false, global, store.Workflow)
	if err != nil {
		return nil, err
	}
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err != nil {
		return nil, err
	}
	data, err := manifest.Marshal(f.Doc)
	if err != nil || out == "" {
		return data, err
	}
	return data, os.WriteFile(out, data, 0o644)
}
