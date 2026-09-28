package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

const renderAddr = "127.0.0.1:4747"

func renderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render <id>[@<version>]",
		Short: "Open a workflow as an HTML page in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")

			scope, err := store.Resolve(global)
			if err != nil {
				return err
			}
			path, _, err := locate(scope, store.Workflow, args[0])
			if err != nil {
				return err
			}
			if err := renderPage(io.Discard, path); err != nil {
				return err
			}
			return serve(path)
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	return cmd
}

func renderPage(w io.Writer, path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
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

// serve re-renders the workflow on every request, so a reload shows the
// latest edit, or why the edit stopped it rendering.
func serve(path string) error {
	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		var buf bytes.Buffer
		if err := renderPage(&buf, path); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, errorPage(path, err))
			return
		}
		buf.WriteTo(w)
	})

	url := "http://" + renderAddr + "/"
	fmt.Printf("%s\n%s\n", path, url)
	go browse(url)
	return http.ListenAndServe(renderAddr, nil)
}

func errorPage(path string, err error) string {
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>awf render: error</title>` +
		`<style>body{margin:0;padding:40px 16px;font:15px/1.5 ui-sans-serif,-apple-system,system-ui,sans-serif;background:#f1f3f6;color:#16202b}` +
		`@media (prefers-color-scheme: dark){body{background:#0e1217;color:#e4e8ee}}` +
		`main{max-width:760px;margin:0 auto}h1{font-size:20px;margin:0 0 6px}p{margin:0 0 16px;color:#8a5300}` +
		`pre{white-space:pre-wrap;font:13px/1.5 ui-monospace,Menlo,monospace;padding:14px;border-radius:8px;background:rgba(127,127,127,.12)}</style>` +
		`<main><h1>This workflow does not render</h1><p>Fix it and reload.</p><pre>` +
		html.EscapeString(path+"\n\n"+err.Error()) + `</pre></main>`
}

func browse(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Run()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
	default:
		exec.Command("xdg-open", url).Run()
	}
}
