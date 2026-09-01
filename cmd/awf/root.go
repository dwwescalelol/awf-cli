package main

import "github.com/spf13/cobra"

var version = "0.1.0"

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "awf",
		Short:         "Operate on OpenAWF workflow documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("awf {{.Version}}\n")
	root.AddCommand(newLs(), newValidate())
	return root
}
