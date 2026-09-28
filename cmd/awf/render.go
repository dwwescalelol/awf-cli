package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func renderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render ([-t] <id>[@<version>] | -f <path>)",
		Short: "Render a workflow or task as an HTML page, served on localhost",
		Long: "Render a workflow or task as a self-contained HTML page.\n\n" +
			"By default the page is served on localhost and re-rendered from the file on every request,\n" +
			"so a reload shows the latest edit. --out writes the page to a file instead, or to stdout with -.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetBool("task")
			out, _ := cmd.Flags().GetString("out")
			addr, _ := cmd.Flags().GetString("addr")
			open, _ := cmd.Flags().GetBool("open")

			kind := store.Workflow
			if task || strings.HasSuffix(file, ".md") {
				kind = store.Task
			}
			path, _, err := target(kind, args, file, global)
			if err != nil {
				return err
			}

			if out != "" {
				return renderTo(out, kind, path)
			}
			// Fail before serving when the document cannot render at all.
			if err := renderPage(io.Discard, kind, path); err != nil {
				return err
			}
			return serve(cmd, addr, open, kind, path)
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.Flags().BoolP("task", "t", false, "render a task by id")
	cmd.Flags().StringP("out", "o", "", "write the page to this file, or to stdout with -")
	cmd.Flags().String("addr", "127.0.0.1:4747", "address to serve on, port 0 picks a free port")
	cmd.Flags().Bool("open", false, "open the page in the default browser")
	return cmd
}

func renderTo(out string, kind store.DocumentKind, path string) error {
	var buf bytes.Buffer
	if err := renderPage(&buf, kind, path); err != nil {
		return err
	}
	if out == "-" {
		_, err := buf.WriteTo(os.Stdout)
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote " + out)
	return nil
}

func renderPage(w io.Writer, kind store.DocumentKind, path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if kind == store.Task {
		t, err := load.Task(path)
		if err != nil {
			return err
		}
		return render.TaskPage(w, path, taskName(path), source, t)
	}
	compiled, err := load.Workflow(path)
	if err != nil {
		return err
	}
	doc, err := manifest.Unmarshal(source)
	if err != nil {
		return err
	}
	return render.Workflow(w, path, source, doc, compiled)
}

// taskName names a task file by its id: the directory it is stored under, or
// the file name outside a store.
func taskName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	dir := filepath.Base(filepath.Dir(abs))
	if filepath.Base(filepath.Dir(filepath.Dir(abs))) == "task" {
		return dir
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func serve(cmd *cobra.Command, addr string, open bool, kind store.DocumentKind, path string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		if err := renderPage(&buf, kind, path); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, errorPage(path, err))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		buf.WriteTo(w)
	})

	fmt.Fprintf(cmd.OutOrStdout(), "serving %s\n%s\n", path, url)
	if open {
		if err := browse(url); err != nil {
			fmt.Fprintln(os.Stderr, "warning: open browser: "+err.Error())
		}
	}
	err = http.Serve(ln, mux)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// errorPage shows why the document stopped rendering, so a reload after an
// edit that breaks it says what broke.
func errorPage(path string, err error) string {
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>awf render: error</title>` +
		`<style>body{margin:0;padding:40px 16px;font:15px/1.5 ui-sans-serif,-apple-system,system-ui,sans-serif;background:#f1f3f6;color:#16202b}` +
		`@media (prefers-color-scheme: dark){body{background:#0e1217;color:#e4e8ee}}` +
		`main{max-width:760px;margin:0 auto}h1{font-size:20px;margin:0 0 6px}p{margin:0 0 16px;color:#8a5300}` +
		`pre{white-space:pre-wrap;font:13px/1.5 ui-monospace,Menlo,monospace;padding:14px;border-radius:8px;background:rgba(127,127,127,.12)}</style>` +
		`<main><h1>This document does not render</h1><p>Fix it and reload.</p><pre>` +
		html.EscapeString(path+"\n\n"+err.Error()) + `</pre></main>`
}

func browse(url string) error {
	name := "xdg-open"
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	return exec.Command(name, url).Start()
}
