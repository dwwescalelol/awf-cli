package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	root := &cobra.Command{
		Use:           "awf",
		Short:         "Operate on OpenAWF workflow documents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       build.Version,
	}
	root.SetVersionTemplate("awf {{.Version}}\n")
	root.PersistentFlags().Bool("global", false, "act on the global store: $AWF_HOME, deafults to ~/.awf")
	root.AddCommand(bundleCmd(), lsCmd(), lspCmd(), newCmd(), renderCmd(), sealCmd(), uiCmd(), validateCmd())

	// Cobra checks --help and flag errors before positional arguments. A bad
	// argument reports first on every path.
	var argErr error
	help := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if argErr = precheck(cmd); argErr == nil {
			help(cmd, args)
		}
	})
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if argErr := precheck(cmd); argErr != nil {
			return argErr
		}
		var missing *pflag.ValueRequiredError
		if errors.As(err, &missing) && missing.GetFlag().Name == "file" {
			return errNoPath
		}
		return err
	})
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		return err
	}
	return argErr
}

func warn(msg string) {
	fmt.Fprintln(os.Stderr, "warning: "+msg)
}
