package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

var errInvalid = errors.New("does not validate, the page shows why")

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

			page, err := renderPage(args, file, task, global)
			if err != nil && !errors.Is(err, errInvalid) {
				return err
			}
			if showErr := show(page, source); showErr != nil {
				return showErr
			}
			return err
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "render a task by id")
	cmd.Flags().Bool("source", false, "print the HTML to stdout instead of opening it")
	return cmd
}

func renderPage(args []string, file string, task, global bool) ([]byte, error) {
	kind := store.KindOf(file)
	if task {
		kind = store.Task
	}
	scope, err := store.Resolve(global)
	if err != nil {
		return nil, err
	}
	path, at, err := target(scope, kind, args, file)
	if err != nil {
		return nil, err
	}
	var page bytes.Buffer
	if kind == store.Task {
		err = renderTask(&page, path, at)
	} else {
		err = renderWorkflow(&page, path, scope, at, render.Scope{}, "")
	}
	return page.Bytes(), err
}

func show(page []byte, source bool) error {
	if source {
		_, err := os.Stdout.Write(page)
		return err
	}
	u, err := writePage(page)
	if err != nil {
		return err
	}
	fmt.Println(u)
	if err := browse(u); err != nil {
		fmt.Fprintln(os.Stderr, "warning: open browser: "+err.Error())
	}
	return nil
}

func writePage(page []byte) (string, error) {
	f, err := os.CreateTemp("", "awf-render-*.html")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(page); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	path := filepath.ToSlash(f.Name())
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String(), nil
}

func renderWorkflow(w io.Writer, path string, s *store.Store, at *stored, scope render.Scope, live string) error {
	f, err := load.ReadWorkflow(path, s)
	if f == nil {
		return err
	}
	if err == nil {
		err = at.check(f.Doc.Name, f.Doc.Version)
	}
	if err != nil {
		return invalid(w, path, render.WorkflowHeader(fileName(path), f.Doc), f.Source, err, scope, live)
	}
	return render.Workflow(w, path, f.Source, f.Doc, f.Compiled, scope, live)
}

func renderTask(w io.Writer, path string, at *stored) error {
	name := fileName(path)
	if at != nil {
		name = at.id.String()
	}
	f, err := load.ReadTask(path)
	if f == nil {
		return err
	}
	if err == nil {
		err = at.check("", f.Doc.Version)
	}
	if err != nil {
		return invalid(w, path, render.TaskHeader(name, f.Doc), f.Source, err, render.Scope{}, "")
	}
	return render.Task(w, path, name, f.Source, f.Doc)
}

func invalid(w io.Writer, path string, h render.Header, source []byte, cause error, scope render.Scope, live string) error {
	if err := render.Invalid(w, path, h, source, cause, scope, live); err != nil {
		return err
	}
	return fmt.Errorf("%s: %w", path, errInvalid)
}

func fileName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
