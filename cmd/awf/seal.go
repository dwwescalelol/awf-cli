package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func sealCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seal ([-t] <id>[@<version>] | -f <path>)",
		Short: "Bundle, validate and hash a document, and write its sha",
		Long: `Bundle, validate and hash a document, and write its sha.

The sha covers the bundled document without its sha and version. A sealed
workflow holds its $ref tasks inline. Seal refuses a sealed document unless
--force is given.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetBool("task")
			force, _ := cmd.Flags().GetBool("force")

			path, sha, err := sealDocument(args, file, task, force, global)
			if err != nil {
				return err
			}
			fmt.Println(path + "\nsealed " + sha)
			return nil
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "seal a task by id")
	cmd.Flags().Bool("force", false, "re-seal a sealed document")
	return cmd
}

func sealDocument(args []string, file string, task, force, global bool) (string, string, error) {
	d, err := resolve(args, file, task, global)
	if err != nil {
		return "", "", err
	}
	var data []byte
	var sha string
	if d.kind == store.Task {
		data, sha, err = sealTask(d, force)
	} else {
		data, sha, err = sealWorkflow(d, force)
	}
	if err != nil {
		return d.path, "", err
	}
	if d.at != nil {
		err = d.scope.Replace(d.kind, d.at.ID, d.at.Version, data)
	} else {
		err = os.WriteFile(d.path, data, 0o644)
	}
	return d.path, sha, err
}

func sealTask(d document, force bool) ([]byte, string, error) {
	f, err := load.ReadTask(d.path, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil, f != nil && f.Doc != nil && f.Doc.SHA != nil, force, err); err != nil {
		return nil, "", err
	}
	f.Doc.SHA = nil
	sha, err := seal.Task(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.MarshalTask(f.Doc)
	return data, sha, err
}

func sealWorkflow(d document, force bool) ([]byte, string, error) {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err := sealable(d.path, f != nil && f.Compiled != nil, f != nil && f.Doc != nil && f.Doc.SHA != nil, force, err); err != nil {
		return nil, "", err
	}
	f.Doc.SHA = nil
	sha, err := seal.Workflow(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.Marshal(f.Doc)
	return data, sha, err
}

func sealable(path string, loaded, sealed, force bool, err error) error {
	if loaded && errors.Is(err, seal.ErrMismatch) {
		err = nil
	}
	if err != nil {
		return err
	}
	if sealed && !force {
		return fmt.Errorf("%s: %w, --force to re-seal", path, seal.ErrSealed)
	}
	return nil
}
