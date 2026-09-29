package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "awf",
		Short:         "Operate on OpenAWF workflow documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.CLI,
	}
	root.SetVersionTemplate("awf {{.Version}}\n")
	root.AddCommand(lsCmd(), newCmd(), renderCmd(), uiCmd(), validateCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatErr(err))
		os.Exit(1)
	}
}
