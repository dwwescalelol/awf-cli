package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "awf",
		Short:         "Operate on OpenAWF workflow documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       build.Version,
	}
	root.SetVersionTemplate("awf {{.Version}}\n")
	root.PersistentFlags().Bool("global", false, "act on the global store: $AWF_HOME, deafults to ~/.awf")
	root.AddCommand(bundleCmd(), lsCmd(), newCmd(), renderCmd(), uiCmd(), validateCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func warn(msg string) {
	fmt.Fprintln(os.Stderr, "warning: "+msg)
}
