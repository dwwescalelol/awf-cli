package ui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func create(t *testing.T, s *store.Store, kind store.DocumentKind, id store.ID) (string, version.Version) {
	t.Helper()
	v, err := scaffold.Create(s, kind, id, version.Version{}, false)
	if err != nil {
		t.Fatal(err)
	}
	return s.Path(kind, id, v), v
}

func rewrite(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s has no %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, h http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func handler(s *store.Store) http.Handler {
	return newHandler(s, func(string) {})
}

func TestEmpty(t *testing.T) {
	rec := get(t, handler(store.New(t.TempDir())), "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "No workflows in scope") {
		t.Errorf("got %d %q, want the empty page", rec.Code, rec.Body.String())
	}
}

func TestWorkflow(t *testing.T) {
	s := store.New(t.TempDir())
	path, v := create(t, s, store.Workflow, "deploy")
	h := handler(s)
	url := workflowURL("deploy", v)

	rec := get(t, h, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != url {
		t.Fatalf("home: got %d to %q, want a redirect to %q", rec.Code, rec.Header().Get("Location"), url)
	}

	rec = get(t, h, url)
	page := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(page, "<h1>deploy</h1>") || !strings.Contains(page, "EventSource") {
		t.Errorf("page: got %d, want the live workflow page", rec.Code)
	}

	rewrite(t, path, "start: start", "start: nowhere")
	rec = get(t, h, url)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `class="overlay"`) {
		t.Errorf("invalid: got %d, want the page with its overlay", rec.Code)
	}

	for _, missing := range []string{"/wf/deploy/9.9.9", "/wf/Not_An_ID/0.1.0", "/wf/deploy/latest"} {
		if rec := get(t, h, missing); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", missing, rec.Code)
		}
	}
}

func TestEvents(t *testing.T) {
	s := store.New(t.TempDir())
	_, v := create(t, s, store.Workflow, "deploy")
	srv := httptest.NewServer(handler(s))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+workflowURL("deploy", v)+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("got %q, want text/event-stream", got)
	}
}

func TestLocalOnly(t *testing.T) {
	h := localOnly("127.0.0.1:4747", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for host, want := range map[string]int{
		"127.0.0.1:4747": http.StatusOK,
		"localhost:4747": http.StatusOK,
		"evil.test:4747": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s: got %d, want %d", host, rec.Code, want)
		}
	}
}

func TestEventsOnRefChange(t *testing.T) {
	s := store.New(t.TempDir())
	task, _ := create(t, s, store.Task, "build")
	wf, v := create(t, s, store.Workflow, "deploy")
	rewrite(t, wf, "start: start", "start: build")
	rewrite(t, wf, "  start: null", "  build: null")
	data, err := os.ReadFile(wf)
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(data), "tasks:")
	if err := os.WriteFile(wf, []byte(head+"tasks:\n  build:\n    $ref: build@0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(handler(s))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+workflowURL("deploy", v)+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(task, later, later); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || line != "data: changed\n" {
		t.Errorf("got %q, %v, want a change event", line, err)
	}
}

func TestLinksFlagsInvalid(t *testing.T) {
	s := store.New(t.TempDir())
	_, good := create(t, s, store.Workflow, "deploy")
	bad, _ := create(t, s, store.Workflow, "deploy")
	rewrite(t, bad, "start: start", "start: nowhere")

	scope, _, err := links(s, &store.Entry{ID: "deploy", Version: good})
	if err != nil {
		t.Fatalf("a malformed sibling failed the listing: %v", err)
	}
	if len(scope.Versions) != 2 || !scope.Versions[0].Invalid || scope.Versions[1].Invalid {
		t.Errorf("versions: got %+v, want the newest flagged invalid", scope.Versions)
	}
}
