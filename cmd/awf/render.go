package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

// errInvalid reports that render wrote the page for a document that does not
// validate. The page itself shows why.
var errInvalid = errors.New("does not validate, the page shows why")

func renderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render ([-t] <id>[@<version>] | -f <path>)",
		Short: "Print a workflow or task as a self-contained HTML page",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetBool("task")

			kind := store.Workflow
			if task || strings.HasSuffix(file, ".md") {
				kind = store.Task
			}
			path, _, err := target(kind, args, file, global)
			if err != nil {
				return err
			}
			out := bufio.NewWriter(os.Stdout)
			if kind == store.Task {
				err = renderTask(out, path)
			} else {
				err = renderWorkflow(out, path, render.Scope{}, "")
			}
			if flushErr := out.Flush(); flushErr != nil {
				return flushErr
			}
			return err
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "render a task by id")
	return cmd
}

// renderWorkflow writes the workflow's page. A workflow that does not
// validate still gets a page, reporting why, and the result is errInvalid.
func renderWorkflow(w io.Writer, path string, scope render.Scope, live string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	compiled, err := load.Workflow(path)
	if err != nil {
		return invalid(w, path, workflowHeader(path, source), source, err, scope, live)
	}
	doc, err := manifest.Unmarshal(source)
	if err != nil {
		return invalid(w, path, workflowHeader(path, source), source, err, scope, live)
	}
	return render.Workflow(w, path, source, doc, compiled, scope, live)
}

// renderTask writes the task's page, or its invalid page as renderWorkflow does.
func renderTask(w io.Writer, path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	name := taskName(path)
	t, err := load.Task(path)
	if err != nil {
		h := render.Header{Name: name}
		if doc, parseErr := manifest.UnmarshalTask(source); parseErr == nil {
			h.Version, h.Summary, h.SHA = versionLabel(doc.Version.String()), doc.Summary, deref(doc.SHA)
		}
		return invalid(w, path, h, source, err, render.Scope{}, "")
	}
	return render.Task(w, path, name, source, t)
}

func invalid(w io.Writer, path string, h render.Header, source []byte, cause error, scope render.Scope, live string) error {
	if err := render.Invalid(w, path, h, source, cause, scope, live); err != nil {
		return err
	}
	return errInvalid
}

// workflowHeader reads what it can of a workflow that does not validate. Its
// name falls back to the file name.
func workflowHeader(path string, source []byte) render.Header {
	h := render.Header{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	doc, err := manifest.Unmarshal(source)
	if err != nil {
		return h
	}
	if doc.Name != "" {
		h.Name = doc.Name
	}
	h.Version, h.Summary, h.SHA = versionLabel(doc.Version.String()), doc.Summary, deref(doc.SHA)
	return h
}

// taskName names a task file by its id: the directory it is stored under, or
// the file name outside a store.
func taskName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if filepath.Base(filepath.Dir(filepath.Dir(abs))) == "task" {
		return filepath.Base(filepath.Dir(abs))
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func versionLabel(v string) string {
	if v == "0.0.0" {
		return ""
	}
	return v
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
