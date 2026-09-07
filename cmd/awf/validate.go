package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate (<id>[@<version>] | -f <path>)",
		Short: "Check a workflow document against the OpenAWF spec",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")

			path, err := validate(args, file, global)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}
			fmt.Println(formatValid(path))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	cmd.Flags().StringP("file", "f", "", "path to a document file")
	return cmd
}

func formatValid(path string) string { return path + "\nvalid" }

func formatErr(err error) string { return err.Error() }

func validate(args []string, file string, global bool) (string, error) {
	path, err := target(args, file, global)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	wf, err := manifest.Parse(data)
	if err != nil {
		return path, err
	}
	if err := wf.Validate(); err != nil {
		return path, err
	}
	return path, wf.Graph()
}

func target(args []string, file string, global bool) (string, error) {
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
	return locate(scope, store.Workflow, args[0])
}

func locate(scope store.Scope, kind store.DocumentKind, ref string) (string, error) {
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
