package main

import (
	"os"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func bundleCmd() *cobra.Command {
	workflow := documentCmd(store.Workflow, "Dereference a workflow's $ref tasks into one document", func(cmd *cobra.Command, r request) error {
		out, _ := cmd.Flags().GetString("out")
		data, err := bundle(r, out)
		if err != nil {
			return err
		}
		if out != "" {
			return nil
		}
		_, err = os.Stdout.Write(data)
		return err
	})
	workflow.Flags().StringP("out", "o", "", "write the bundled workflow to this path")
	return kindCmd(&cobra.Command{
		Use:   "bundle",
		Short: "Dereference a workflow's $ref tasks into one document",
		Long: `Dereference a workflow's $ref tasks into one document.

A $ref is a path to a task file or a stored task pinned as <id>@<version>.
A relative path resolves against the directory holding the store in scope.
An id resolves in the store in scope. The bundled workflow prints to stdout,
or to the file given by -o.`,
	}, workflow)
}

func bundle(r request, out string) ([]byte, error) {
	d, err := resolve(r)
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
