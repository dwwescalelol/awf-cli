package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dwwescalelol/awf-cli/internal/browser"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/ui"
	"github.com/spf13/cobra"
)

const defaultPort = 4747

func uiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Browse the workflows in scope in the browser",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			global, _ := cmd.Flags().GetBool("global")
			port, _ := cmd.Flags().GetInt("port")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			scope, err := store.Resolve(global)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ln, err := ui.Listen(port, !cmd.Flags().Changed("port"))
			if err != nil {
				return err
			}
			u := "http://" + ln.Addr().String() + "/"
			fmt.Printf("%s\n%s\n", scope.Dir(), u)
			if !noBrowser {
				if err := browser.Open(u); err != nil {
					warn("open browser: " + err.Error())
				}
			}
			return ui.Serve(ctx, ln, scope, warn)
		},
	}
	cmd.Flags().IntP("port", "p", defaultPort, "port to serve on, 0 picks a free one")
	cmd.Flags().Bool("no-browser", false, "print the URL without opening it")
	return cmd
}
