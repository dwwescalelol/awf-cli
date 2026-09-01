package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/dwwescalelol/awf-cli/internal/validate"
	"github.com/spf13/cobra"
)

type report struct {
	path   string
	result validate.Result
}

var errInvalid = errors.New("document is not valid")

func newValidate() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Check a workflow document against the OpenAWF spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := validateFile(args[0], strict)
			if err != nil {
				return err
			}
			printReport(cmd.OutOrStdout(), r)
			if r.result.Failed() {
				return errInvalid
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as errors")
	return cmd
}

func validateFile(path string, strict bool) (report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return report{}, err
	}
	return report{path: path, result: validate.Document(data, strict)}, nil
}

func printReport(out io.Writer, r report) {
	fmt.Fprintln(out, r.path)
	if len(r.result.Unresolved) > 0 {
		fmt.Fprintf(out, "unbundled $ref tasks, their outcomes and uses are unchecked: %s\n",
			strings.Join(r.result.Unresolved, ", "))
	}
	if len(r.result.Diagnostics) == 0 {
		fmt.Fprintln(out, "valid")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, d := range r.result.Diagnostics {
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.Severity, d.Path, d.Message)
	}
	w.Flush()
}
