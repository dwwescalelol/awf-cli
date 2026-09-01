package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/spf13/cobra"
)

var errInvalid = errors.New("document is not valid")

func newValidate() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "validate <path>|<id>@<version>",
		Short: "Check a workflow document against the OpenAWF spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := locate(args[0], global)
			if err != nil {
				return err
			}
			return validateFile(cmd.OutOrStdout(), path)
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "act on ~/.awf")
	return cmd
}

func validateFile(out io.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, path)

	wf, err := manifest.Parse(data)
	if err != nil {
		fmt.Fprintln(out, err)
		return errInvalid
	}
	if err := wf.Validate(); err != nil {
		fmt.Fprintln(out, err)
		return errInvalid
	}

	stuck := wf.Build().StuckStates()
	for _, s := range stuck {
		fmt.Fprintf(out, "/orchestration/%s: the flow can never leave this task\n", s.Task.Name)
	}
	if len(stuck) > 0 {
		return errInvalid
	}

	fmt.Fprintln(out, "valid")
	return nil
}
