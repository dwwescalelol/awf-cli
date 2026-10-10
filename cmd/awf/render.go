package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/browser"
	"github.com/dwwescalelol/awf-cli/internal/page"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func renderCmd() *cobra.Command {
	run := func(cmd *cobra.Command, r request) error {
		source, _ := cmd.Flags().GetBool("source")
		data, err := renderPage(r)
		if err != nil && !errors.Is(err, page.ErrInvalid) {
			return err
		}
		if showErr := show(data, source); showErr != nil {
			return showErr
		}
		return err
	}
	cmd := kindCmd(&cobra.Command{
		Use:   "render",
		Short: "Open a workflow or task as an HTML page in the browser",
		Long:  "Open a workflow or task as an HTML page in the browser. The page is written to a temporary file, which is left for the browser to read.",
	},
		documentCmd(store.Workflow, "Open a workflow as an HTML page in the browser", run),
		documentCmd(store.Task, "Open a task as an HTML page in the browser", run),
	)
	cmd.PersistentFlags().Bool("source", false, "print the HTML to stdout instead of opening it")
	return cmd
}

func renderPage(r request) ([]byte, error) {
	d, err := resolve(r)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(d.path)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = page.Document(&out, d.kind, d.path, data, d.scope, d.at, render.Scope{}, "", nil)
	return out.Bytes(), err
}

func show(data []byte, source bool) error {
	if source {
		_, err := os.Stdout.Write(data)
		return err
	}
	u, err := browser.WritePage(data)
	if err != nil {
		return err
	}
	fmt.Println(u)
	if err := browser.Open(u); err != nil {
		warn("open browser: " + err.Error())
	}
	return nil
}
