package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <path>|<id>@<version>",
		Short: "Check a workflow document against the OpenAWF spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")

			path, err := validateFile(args[0], global)
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

func validateFile(arg string, global bool) (string, error) {
	path, err := locate(arg, global)
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
