package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func uiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Browse the workflows in scope in the browser",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			global, _ := cmd.Flags().GetBool("global")
			port, _ := cmd.Flags().GetInt("port")
			scope, err := store.Resolve(global)
			if err != nil {
				return err
			}
			return serve(scope, port)
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().IntP("port", "p", 4747, "port to serve on, 0 picks a free one")
	return cmd
}

func workflowURL(id store.ID, v version.Version) string {
	return "/wf/" + id.String() + "/" + v.String()
}

// links lists every workflow in scope, each linking to its latest version,
// and every version of the workflow on the page, newest first.
func links(s *store.Store, current stored) (render.Scope, error) {
	documents, _, err := s.List(store.Workflow)
	if err != nil {
		return render.Scope{}, err
	}
	scope := render.Scope{Dir: s.Dir()}
	for _, d := range documents {
		scope.Workflows = append(scope.Workflows, render.Link{
			Label:   d.ID.String(),
			URL:     workflowURL(d.ID, d.Versions[len(d.Versions)-1]),
			Current: d.ID == current.id,
			Count:   len(d.Versions),
		})
		if d.ID != current.id {
			continue
		}
		for i, v := range slices.Backward(d.Versions) {
			// A version that does not validate is flagged, not fatal: the page
			// for any other version still renders.
			sealed, err := status(s, d.ID, v)
			scope.Versions = append(scope.Versions, render.Link{
				Label:   v.String(),
				URL:     workflowURL(d.ID, v),
				Current: v.Compare(current.version) == 0,
				Latest:  i == len(d.Versions)-1,
				Sealed:  sealed,
				Invalid: err != nil,
			})
		}
	}
	return scope, nil
}

// status reports whether a version is sealed, or why it does not validate.
func status(s *store.Store, id store.ID, v version.Version) (bool, error) {
	path, err := s.Find(store.Workflow, id, v)
	if err != nil {
		return false, err
	}
	if _, err := load.Workflow(path); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	doc, err := manifest.Unmarshal(data)
	if err != nil {
		return false, err
	}
	return doc.SHA != nil, nil
}

// serve renders each page on request. Every page also listens for changes to
// its file and reloads itself, so an edit shows without a manual reload.
func serve(s *store.Store, port int) error {
	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		documents, _, err := s.List(store.Workflow)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(documents) == 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, messagePage("No workflows in scope", "Create one with awf new workflow <id>, then reload.", s.Dir()))
			return
		}
		d := documents[0]
		http.Redirect(w, r, workflowURL(d.ID, d.Versions[len(d.Versions)-1]), http.StatusFound)
	})
	http.HandleFunc("GET /wf/{id}/{version}", func(w http.ResponseWriter, r *http.Request) {
		path, at, ok := find(s, r)
		if !ok {
			http.NotFound(w, r)
			return
		}
		live := workflowURL(at.id, at.version) + "/events"

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		var buf bytes.Buffer
		scope, err := links(s, at)
		if err == nil {
			err = renderWorkflow(&buf, path, scope, live)
		}
		// An invalid workflow still has a page, which reports why.
		if errors.Is(err, errInvalid) {
			err = nil
		}
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, messagePage("This workflow does not render", "Fix the file and the page reloads.", path+"\n\n"+err.Error())+liveScript(live))
			return
		}
		buf.WriteTo(w)
	})
	http.HandleFunc("GET /wf/{id}/{version}/events", func(w http.ResponseWriter, r *http.Request) {
		path, _, ok := find(s, r)
		if !ok {
			http.NotFound(w, r)
			return
		}
		watch(w, r, path)
	})

	// Bind before opening the browser, so a port in use fails here rather than
	// opening whatever already listens on it.
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("%s\n%s\n", s.Dir(), url)
	go browse(url)
	return http.Serve(ln, nil)
}

func find(s *store.Store, r *http.Request) (string, stored, bool) {
	id, err := store.NewID(r.PathValue("id"))
	if err != nil {
		return "", stored{}, false
	}
	v, err := version.Parse(r.PathValue("version"))
	if err != nil {
		return "", stored{}, false
	}
	path, err := s.Find(store.Workflow, id, v)
	if err != nil {
		return "", stored{}, false
	}
	return path, stored{id: id, version: v}, true
}

// watch streams an event each time the file at path changes, checking its
// modification time twice a second until the page closes.
func watch(w http.ResponseWriter, r *http.Request, path string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	flusher.Flush()

	modified := func() time.Time {
		fi, err := os.Stat(path)
		if err != nil {
			return time.Time{}
		}
		return fi.ModTime()
	}
	last := modified()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if now := modified(); !now.Equal(last) {
				last = now
				fmt.Fprint(w, "data: changed\n\n")
				flusher.Flush()
			}
		}
	}
}

func liveScript(url string) string {
	return `<script>new EventSource("` + html.EscapeString(url) + `").onmessage = () => location.reload();</script>`
}

func messagePage(title, hint, detail string) string {
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="icon" href="data:,"><title>awf: ` + html.EscapeString(title) + `</title>` +
		`<style>body{margin:0;padding:40px 16px;font:15px/1.5 ui-sans-serif,-apple-system,system-ui,sans-serif;background:#f1f3f6;color:#16202b}` +
		`@media (prefers-color-scheme: dark){body{background:#0e1217;color:#e4e8ee}}` +
		`main{max-width:760px;margin:0 auto}h1{font-size:20px;margin:0 0 6px}p{margin:0 0 16px;color:#8a5300}` +
		`pre{white-space:pre-wrap;font:13px/1.5 ui-monospace,Menlo,monospace;padding:14px;border-radius:8px;background:rgba(127,127,127,.12)}</style>` +
		`<main><h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(hint) + `</p><pre>` +
		html.EscapeString(detail) + `</pre></main>`
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
