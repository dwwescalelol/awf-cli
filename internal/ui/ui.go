package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/page"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func Serve(ctx context.Context, ln net.Listener, s *store.Store, warn func(string)) error {
	srv := &http.Server{
		Handler:           localOnly(ln.Addr().String(), newHandler(s, warn)),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
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

func Listen(port int, fallback bool) (net.Listener, error) {
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
	report func(string)
	mu     sync.Mutex
	warned map[string]bool
}

func newHandler(s *store.Store, warn func(string)) http.Handler {
	srv := &server{store: s, report: warn, warned: map[string]bool{}}
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
	at, err := srv.entry(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	live := workflowURL(at.ID, at.Version) + "/events"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	scope, skipped, err := links(srv.store, at)
	srv.warn(skipped)
	var data []byte
	if err == nil {
		data, err = os.ReadFile(at.Path)
	}
	if err == nil {
		err = page.Document(w, store.Workflow, at.Path, data, srv.store, at, scope, live, nil)
	}
	if err == nil || errors.Is(err, page.ErrInvalid) {
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	render.Message(w, "This workflow does not render", "Fix the file and the page reloads.", at.Path+"\n\n"+err.Error(), live)
}

func (srv *server) events(w http.ResponseWriter, r *http.Request) {
	at, err := srv.entry(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	paths := []string{at.Path}
	if f, _ := load.ReadWorkflow(at.Path, srv.store, nil); f != nil {
		paths = append(paths, f.Refs...)
	}
	watch(w, r, paths)
}

func (srv *server) entry(r *http.Request) (*store.Entry, error) {
	id, err := store.NewID(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	v, err := version.Parse(r.PathValue("version"))
	if err != nil {
		return nil, err
	}
	path, err := srv.store.Find(store.Workflow, id, v)
	if err != nil {
		return nil, err
	}
	return &store.Entry{Path: path, ID: id, Version: v}, nil
}

func (srv *server) warn(skipped []store.Skipped) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, sk := range skipped {
		if !srv.warned[sk.String()] {
			srv.warned[sk.String()] = true
			srv.report(sk.String())
		}
	}
}

func workflowURL(id store.ID, v version.Version) string {
	return "/wf/" + id.String() + "/" + v.String()
}

func links(s *store.Store, current *store.Entry) (render.Scope, []store.Skipped, error) {
	documents, skipped, err := s.List(store.Workflow)
	if err != nil {
		return render.Scope{}, skipped, err
	}
	scope := render.Scope{Dir: s.Dir()}
	for _, d := range documents {
		scope.Workflows = append(scope.Workflows, render.Link{
			Label:   d.ID.String(),
			URL:     workflowURL(d.ID, d.Versions[len(d.Versions)-1]),
			Current: d.ID == current.ID,
			Count:   len(d.Versions),
		})
		if d.ID != current.ID {
			continue
		}
		for i, v := range slices.Backward(d.Versions) {
			sealed, err := status(s, d.ID, v)
			scope.Versions = append(scope.Versions, render.Link{
				Label:   v.String(),
				URL:     workflowURL(d.ID, v),
				Current: v.Compare(current.Version) == 0,
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
	f, err := load.ReadWorkflow(path, s, nil)
	if err != nil {
		return false, err
	}
	return f.Doc.SHA != nil, nil
}

func watch(w http.ResponseWriter, r *http.Request, paths []string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	flusher.Flush()

	modified := func() string {
		var stamp string
		for _, path := range paths {
			if fi, err := os.Stat(path); err == nil {
				stamp += fi.ModTime().String()
			}
			stamp += "\n"
		}
		return stamp
	}
	last := modified()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if now := modified(); now != last {
				last = now
				fmt.Fprint(w, "data: changed\n\n")
				flusher.Flush()
			}
		}
	}
}
