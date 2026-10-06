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
	cmd := &cobra.Command{
		Use:   "render ([-t] <id>[@<version>] | -f <path>)",
		Short: "Open a workflow or task as an HTML page in the browser",
		Long:  "Open a workflow or task as an HTML page in the browser. The page is written to a temporary file, which is left for the browser to read.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetBool("task")
			source, _ := cmd.Flags().GetBool("source")

			data, err := renderPage(args, file, task, global)
			if err != nil && !errors.Is(err, page.ErrInvalid) {
				return err
			}
			if showErr := show(data, source); showErr != nil {
				return showErr
			}
			return err
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "render a task by id")
	cmd.Flags().Bool("source", false, "print the HTML to stdout instead of opening it")
	return cmd
}

func renderPage(args []string, file string, task, global bool) ([]byte, error) {
	d, err := resolve(args, file, task, global)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if d.kind == store.Task {
		err = page.Task(&out, d.path, d.at)
	} else {
		err = page.Workflow(&out, d.path, d.scope, d.at, render.Scope{}, "")
	}
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
