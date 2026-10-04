package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func get(t *testing.T, h http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func handler(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.Resolve(false)
	if err != nil {
		t.Fatal(err)
	}
	return newHandler(s)
}

func TestUIEmpty(t *testing.T) {
	project(t)
	rec := get(t, handler(t), "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "No workflows in scope") {
		t.Errorf("got %d %q, want the empty page", rec.Code, rec.Body.String())
	}
}

func TestUIWorkflow(t *testing.T) {
	project(t)
	wf, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	h := handler(t)
	url := workflowURL("deploy", wf.version)

	rec := get(t, h, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != url {
		t.Fatalf("home: got %d to %q, want a redirect to %q", rec.Code, rec.Header().Get("Location"), url)
	}

	rec = get(t, h, url)
	page := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(page, "<h1>deploy</h1>") || !strings.Contains(page, "EventSource") {
		t.Errorf("page: got %d, want the live workflow page", rec.Code)
	}

	rewrite(t, wf.path, "start: start", "start: nowhere")
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

func TestUIEvents(t *testing.T) {
	project(t)
	wf, err := create(store.Workflow, "deploy", "", false)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler(t))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+workflowURL("deploy", wf.version)+"/events", nil)
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

func TestUILocalOnly(t *testing.T) {
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
