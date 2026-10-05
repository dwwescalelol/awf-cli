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
		Short: "Inline a workflow's $ref tasks into one document",
		Long: `Inline a workflow's $ref tasks into one document.

A $ref is a path to a task file or a stored task pinned as <id>@<version>.
A relative path resolves against the workflow's directory. An id resolves in
the store in scope. The bundled workflow prints to stdout, or to the file
given by -o.`,
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
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a workflow file")
	cmd.Flags().StringP("out", "o", "", "write the bundled workflow to this path")
	return cmd
}

func bundle(args []string, file, out string, global bool) ([]byte, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return nil, err
	}
	path, _, err := target(scope, store.Workflow, args, file)
	if err != nil {
		return nil, err
	}
	doc, err := load.Document(path, scope)
	if err != nil {
		return nil, err
	}
	data, err := manifest.Marshal(doc)
	if err != nil || out == "" {
		return data, err
	}
	return data, os.WriteFile(out, data, 0o644)
}
