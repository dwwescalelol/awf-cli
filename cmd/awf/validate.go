package main

import (
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
		Use:   "validate <path>|<id>@<version>",
		Short: "Check a workflow document against the OpenAWF spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")

			path, err := validate(args[0], global)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}
			fmt.Println(formatValid(path))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	return cmd
}

func formatValid(path string) string { return path + "\nvalid" }

func formatErr(err error) string { return err.Error() }

func validate(ref string, global bool) (string, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return "", err
	}

	path, err := locate(scope, store.Workflow, ref)
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

func locate(scope store.Scope, kind store.DocumentKind, ref string) (string, error) {
	if _, err := os.Stat(ref); err == nil {
		return ref, nil
	}

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
