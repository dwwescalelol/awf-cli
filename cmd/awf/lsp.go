package main

import (
	"os"

	"github.com/dwwescalelol/awf-cli/internal/lsp"
	"github.com/spf13/cobra"
)

func lspCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Run the OpenAWF language server on stdin and stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return lsp.Serve(os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().Bool("stdio", true, "serve over stdin and stdout")
	cmd.Flags().MarkHidden("stdio")
	return cmd
}
