package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
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
			return serve(ctx, scope, port, !cmd.Flags().Changed("port"), !noBrowser)
		},
	}
	cmd.Flags().Bool("global", false, "act on the global store: $AWF_HOME, or ~/.awf when unset")
	cmd.Flags().IntP("port", "p", defaultPort, "port to serve on, 0 picks a free one")
	cmd.Flags().Bool("no-browser", false, "print the URL without opening it")
	return cmd
}

func serve(ctx context.Context, s *store.Store, port int, fallback, open bool) error {
	ln, err := listen(port, fallback)
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	srv := &http.Server{
		Handler:           localOnly(addr, newHandler(s)),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	u := "http://" + addr + "/"
	fmt.Printf("%s\n%s\n", s.Dir(), u)
	if open {
		if err := browse(u); err != nil {
			fmt.Fprintln(os.Stderr, "warning: open browser: "+err.Error())
		}
	}

	done := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-done
}

func listen(port int, fallback bool) (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil && fallback {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, err
}

func localOnly(addr string, next http.Handler) http.Handler {
	_, port, _ := net.SplitHostPort(addr)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != addr && r.Host != "localhost:"+port {
			http.Error(w, "host "+r.Host+" not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type server struct {
	store  *store.Store
	mu     sync.Mutex
	warned map[string]bool
}

func newHandler(s *store.Store) http.Handler {
	srv := &server{store: s, warned: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.home)
	mux.HandleFunc("GET /wf/{id}/{version}", srv.workflow)
	mux.HandleFunc("GET /wf/{id}/{version}/events", srv.events)
	return mux
}

func (srv *server) home(w http.ResponseWriter, r *http.Request) {
	documents, skipped, err := srv.store.List(store.Workflow)
	srv.warn(skipped)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(documents) == 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render.Message(w, "No workflows in scope", "Create one with awf new workflow <id>, then reload.", srv.store.Dir(), "")
		return
	}
	d := documents[0]
	http.Redirect(w, r, workflowURL(d.ID, d.Versions[len(d.Versions)-1]), http.StatusFound)
}

func (srv *server) workflow(w http.ResponseWriter, r *http.Request) {
	path, at, err := find(srv.store, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	live := workflowURL(at.id, at.version) + "/events"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	scope, skipped, err := links(srv.store, at)
	srv.warn(skipped)
	if err == nil {
		err = renderWorkflow(w, path, srv.store, &at, scope, live)
	}
	if err == nil || errors.Is(err, errInvalid) {
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	render.Message(w, "This workflow does not render", "Fix the file and the page reloads.", path+"\n\n"+err.Error(), live)
}

func (srv *server) events(w http.ResponseWriter, r *http.Request) {
	path, _, err := find(srv.store, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	watch(w, r, path)
}

func (srv *server) warn(skipped []store.Skipped) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, sk := range skipped {
		if !srv.warned[sk.String()] {
			srv.warned[sk.String()] = true
			fmt.Fprintln(os.Stderr, "warning: "+sk.String())
		}
	}
}

func workflowURL(id store.ID, v version.Version) string {
	return "/wf/" + id.String() + "/" + v.String()
}

func links(s *store.Store, current stored) (render.Scope, []store.Skipped, error) {
	documents, skipped, err := s.List(store.Workflow)
	if err != nil {
		return render.Scope{}, skipped, err
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
	return scope, skipped, nil
}

func status(s *store.Store, id store.ID, v version.Version) (bool, error) {
	path, err := s.Find(store.Workflow, id, v)
	if err != nil {
		return false, err
	}
	f, err := load.ReadWorkflow(path, s)
	if err != nil {
		return false, err
	}
	return f.Doc.SHA != nil, nil
}

func find(s *store.Store, r *http.Request) (string, stored, error) {
	id, err := store.NewID(r.PathValue("id"))
	if err != nil {
		return "", stored{}, err
	}
	v, err := version.Parse(r.PathValue("version"))
	if err != nil {
		return "", stored{}, err
	}
	path, err := s.Find(store.Workflow, id, v)
	if err != nil {
		return "", stored{}, err
	}
	return path, stored{id: id, version: v}, nil
}

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

func browse(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}
