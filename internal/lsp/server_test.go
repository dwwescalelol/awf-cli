package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

type client struct {
	t    *testing.T
	conn *conn
	id   int
	in   chan map[string]json.RawMessage
	done chan error
}

func start(t *testing.T) *client {
	t.Helper()
	toServer, fromClient := io.Pipe()
	fromServer, toClient := io.Pipe()
	c := &client{t: t, conn: &conn{in: bufio.NewReader(fromServer), out: fromClient}, in: make(chan map[string]json.RawMessage, 16), done: make(chan error, 1)}
	go func() {
		c.done <- Serve(toServer, toClient)
		toClient.Close()
	}()
	go func() {
		defer close(c.in)
		for {
			body, err := c.conn.read()
			if err != nil {
				return
			}
			var m map[string]json.RawMessage
			if json.Unmarshal(body, &m) != nil {
				return
			}
			c.in <- m
		}
	}()
	t.Cleanup(func() { fromClient.Close() })
	return c
}

func (c *client) send(v map[string]any) {
	c.t.Helper()
	v["jsonrpc"] = "2.0"
	if err := c.conn.write(v); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) notify(method string, params any) {
	c.t.Helper()
	c.send(map[string]any{"method": method, "params": params})
}

func (c *client) call(method string, params any) map[string]json.RawMessage {
	c.t.Helper()
	c.id++
	c.send(map[string]any{"id": c.id, "method": method, "params": params})
	return c.recv()
}

func (c *client) recv() map[string]json.RawMessage {
	c.t.Helper()
	select {
	case m, ok := <-c.in:
		if !ok {
			c.t.Fatal("server closed the connection")
		}
		return m
	case <-time.After(5 * time.Second):
		c.t.Fatal("timed out waiting for the server")
	}
	return nil
}

func (c *client) initialize() {
	c.t.Helper()
	c.call("initialize", map[string]any{"capabilities": map[string]any{}})
	c.notify("initialized", map[string]any{})
}

func (c *client) diagnostics() publishParams {
	c.t.Helper()
	m := c.recv()
	if string(m["method"]) != `"textDocument/publishDiagnostics"` {
		c.t.Fatalf("got %v, want publishDiagnostics", m)
	}
	var p publishParams
	if err := json.Unmarshal(m["params"], &p); err != nil {
		c.t.Fatal(err)
	}
	return p
}

func (c *client) open(path, text string) {
	c.t.Helper()
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": fileURI(path), "version": 1, "text": text}})
}

func decode[T any](t *testing.T, m map[string]json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(m["result"], &v); err != nil {
		t.Fatalf("got %v: %v", m, err)
	}
	return v
}

type fixture struct {
	wf   string
	task string
	text string
}

func setup(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv(store.EnvHome, filepath.Join(root, "home"))
	s := store.New(filepath.Join(root, ".awf"))
	f := fixture{wf: create(t, s, store.Workflow, "deploy"), task: create(t, s, store.Task, "build")}
	f.text = strings.Replace(read(t, f.wf), "start: null", "start: build\n  build: null", 1) + "  build:\n    $ref: build@0.1.0\n"
	return f
}

func create(t *testing.T, s *store.Store, kind store.DocumentKind, id store.ID) string {
	t.Helper()
	v, err := scaffold.Create(s, kind, id, version.Version{}, false)
	if err != nil {
		t.Fatal(err)
	}
	return s.Path(kind, id, v)
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func at(text, needle string) textRange {
	i := strings.Index(text, needle)
	line := strings.Count(text[:i], "\n")
	col := i - strings.LastIndex(text[:i], "\n") - 1
	return textRange{Start: position{Line: line, Character: col}, End: position{Line: line, Character: col + len(needle)}}
}

func TestWorkflowDiagnostics(t *testing.T) {
	f := setup(t)
	c := start(t)
	c.initialize()

	broken := strings.Replace(f.text, "start: start", "start: nowhere", 1)
	c.open(f.wf, broken)
	got := c.diagnostics()
	if got.URI != fileURI(f.wf) || got.Version != 1 || len(got.Diagnostics) != 1 {
		t.Fatalf("got %+v, want one diagnostic on %s version 1", got, fileURI(f.wf))
	}
	if d := got.Diagnostics[0]; d.Range != at(broken, "nowhere") || d.Severity != severityError {
		t.Errorf("got %+v, want an error on the start value", d)
	}

	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": fileURI(f.wf), "version": 2}, "contentChanges": []any{map[string]any{"text": f.text}}})
	if got := c.diagnostics(); got.Version != 2 || len(got.Diagnostics) != 0 {
		t.Errorf("got %+v, want none at version 2", got)
	}
}

func TestTaskDiagnostics(t *testing.T) {
	f := setup(t)
	c := start(t)
	c.initialize()

	text := strings.Replace(read(t, f.task), "summary:", "bogus: 1\nsummary:", 1)
	c.open(f.task, text)
	got := c.diagnostics()
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Range != at(text, "bogus: 1") {
		t.Fatalf("got %+v, want one diagnostic on the bogus key", got)
	}

	c.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": fileURI(f.task)}})
	if got := c.diagnostics(); got.URI != fileURI(f.task) || len(got.Diagnostics) != 0 {
		t.Errorf("didClose: got %+v, want none", got)
	}
}

func TestDependentRefresh(t *testing.T) {
	f := setup(t)
	c := start(t)
	c.initialize()

	c.open(f.wf, f.text)
	c.diagnostics()
	c.open(f.task, read(t, f.task))
	c.diagnostics()
	c.diagnostics()

	broken := strings.Replace(read(t, f.task), "summary:", "bogus: 1\nsummary:", 1)
	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": fileURI(f.task), "version": 2}, "contentChanges": []any{map[string]any{"text": broken}}})
	published := map[string]publishParams{}
	for range 2 {
		p := c.diagnostics()
		published[p.URI] = p
	}
	got := published[fileURI(f.wf)].Diagnostics
	if len(got) != 1 || got[0].Range != at(f.text, "build@0.1.0") || len(got[0].Related) != 1 {
		t.Fatalf("got %+v, want one diagnostic on the $ref with its location", got)
	}
	rel := got[0].Related[0]
	path, err := filePath(rel.Location.URI)
	if err != nil || canonical(path) != canonical(f.task) || rel.Location.Range != at(broken, "bogus: 1") || rel.Message != got[0].Message {
		t.Errorf("got %+v, want the bogus key in %s", rel, f.task)
	}
}

func TestRender(t *testing.T) {
	f := setup(t)
	notes := filepath.Join(filepath.Dir(f.wf), "notes.yaml")
	if err := os.WriteFile(notes, []byte("title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := start(t)
	c.initialize()

	html := decode[map[string]string](t, c.call(methodRender, map[string]any{"uri": fileURI(f.wf)}))["html"]
	if !strings.Contains(html, "<h1>deploy</h1>") || strings.Contains(html, `class="overlay"`) {
		t.Errorf("render: got no valid page for %s", f.wf)
	}

	c.open(f.wf, strings.Replace(f.text, "start: start", "start: nowhere", 1))
	c.diagnostics()
	html = decode[map[string]string](t, c.call(methodRender, map[string]any{"uri": fileURI(f.wf)}))["html"]
	if !strings.Contains(html, `class="overlay"`) || !strings.Contains(html, "nowhere") {
		t.Errorf("render invalid: got no overlay from the open buffer")
	}

	resp := c.call(methodRender, map[string]any{"uri": fileURI(notes)})
	if !strings.Contains(string(resp["error"]), "-32803") || !strings.Contains(string(resp["error"]), ErrNotOpenAWF.Error()) {
		t.Errorf("render other: got %s, want %v", resp, ErrNotOpenAWF)
	}

	resp = c.call(methodRender, map[string]any{"uri": "untitled:Untitled-1"})
	if !strings.Contains(string(resp["error"]), "-32602") {
		t.Errorf("render untitled: got %s, want invalid params", resp)
	}
}

func TestDefinition(t *testing.T) {
	f := setup(t)
	c := start(t)
	c.initialize()
	c.open(f.wf, f.text)
	c.diagnostics()

	ref := at(f.text, "build@0.1.0")
	locs := decode[[]location](t, c.call("textDocument/definition", map[string]any{"textDocument": map[string]any{"uri": fileURI(f.wf)}, "position": ref.End}))
	if len(locs) != 1 {
		t.Fatalf("got %+v, want one location", locs)
	}
	if path, err := filePath(locs[0].URI); err != nil || !filepath.IsAbs(path) || canonical(path) != canonical(f.task) {
		t.Errorf("got %s, want %s", locs[0].URI, f.task)
	}

	resp := c.call("textDocument/definition", map[string]any{"textDocument": map[string]any{"uri": fileURI(f.wf)}, "position": position{}})
	if string(resp["result"]) != "null" {
		t.Errorf("off a $ref: got %s, want null", resp["result"])
	}
}

func TestCodeLens(t *testing.T) {
	f := setup(t)
	notes := filepath.Join(filepath.Dir(f.wf), "notes.yaml")
	if err := os.WriteFile(notes, []byte("title: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := start(t)
	c.initialize()

	type lens struct {
		Range   textRange
		Command struct {
			Title     string
			Command   string
			Arguments []string
		}
	}
	for _, path := range []string{f.wf, f.task} {
		lenses := decode[[]lens](t, c.call("textDocument/codeLens", map[string]any{"textDocument": map[string]any{"uri": fileURI(path)}}))
		if len(lenses) != 1 || lenses[0].Range != (textRange{}) || lenses[0].Command.Command != "awf.preview" || len(lenses[0].Command.Arguments) != 1 || lenses[0].Command.Arguments[0] != fileURI(path) {
			t.Errorf("%s: got %+v, want one awf.preview lens", path, lenses)
		}
	}
	if resp := c.call("textDocument/codeLens", map[string]any{"textDocument": map[string]any{"uri": fileURI(notes)}}); string(resp["result"]) != "null" {
		t.Errorf("notes.yaml: got %s, want null", resp["result"])
	}
}

func TestLifecycle(t *testing.T) {
	c := start(t)
	if resp := c.call("textDocument/codeLens", map[string]any{}); !strings.Contains(string(resp["error"]), "-32002") {
		t.Fatalf("before initialize: got %s, want -32002", resp)
	}
	init := c.call("initialize", map[string]any{"capabilities": map[string]any{}})
	for _, want := range []string{`"definitionProvider":true`, `"codeLensProvider":{}`, `"save":true`} {
		if !strings.Contains(string(init["result"]), want) {
			t.Errorf("initialize: got %s, want %s", init["result"], want)
		}
	}
	c.send(map[string]any{"id": 99, "result": nil})
	if resp := c.call("nope", nil); !strings.Contains(string(resp["error"]), "-32601") {
		t.Errorf("unknown method: got %s", resp)
	}
	c.call("shutdown", nil)
	if resp := c.call("textDocument/codeLens", map[string]any{}); !strings.Contains(string(resp["error"]), "-32600") {
		t.Errorf("after shutdown: got %s, want -32600", resp)
	}
	c.notify("exit", nil)
	if err := <-c.done; err != nil {
		t.Errorf("Serve: %v", err)
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	c := start(t)
	c.initialize()
	c.notify("exit", nil)
	if err := <-c.done; err != ErrNoShutdown {
		t.Errorf("got %v, want %v", err, ErrNoShutdown)
	}
}
