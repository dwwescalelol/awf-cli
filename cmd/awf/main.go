package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	root := &cobra.Command{
		Use:           "awf",
		Short:         "Operate on OpenAWF workflow documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("awf {{.Version}}\n")
	root.AddCommand(lsCmd(), validateCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
