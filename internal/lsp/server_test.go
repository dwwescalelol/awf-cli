package lsp

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validWorkflow = `openawf: 0.1.0
name: demo
version: 0.1.0
sha: null
start: plan
orchestration:
  plan: ship
  ship: null
tasks:
  plan:
    body: plan
  ship:
    $ref: ship.md
`

const (
	validTask  = "---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\nsummary: x\n---\n\nbody\n"
	brokenTask = "---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\nsummary: x\nbogus: 1\n---\n\nbody\n"
)

type client struct {
	t    *testing.T
	conn *conn
	id   int
}

func (c *client) send(method string, params any) {
	c.t.Helper()
	if err := c.conn.write(notification{JSONRPC: "2.0", Method: method, Params: params}); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) call(method string, params any) map[string]json.RawMessage {
	c.t.Helper()
	c.id++
	req := map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method, "params": params}
	if err := c.conn.write(req); err != nil {
		c.t.Fatal(err)
	}
	return c.recv()
}

func (c *client) recv() map[string]json.RawMessage {
	c.t.Helper()
	body, err := c.conn.read()
	if err != nil {
		c.t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		c.t.Fatal(err)
	}
	return m
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

func start(t *testing.T) (*client, chan error) {
	t.Helper()
	toServer, fromClient := io.Pipe()
	fromServer, toClient := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- Serve(toServer, toClient)
		toClient.Close()
	}()
	t.Cleanup(func() { fromClient.Close() })
	return &client{t: t, conn: newConn(fromServer, fromClient)}, done
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWF_HOME", filepath.Join(dir, "home"))
	if err := os.Mkdir(filepath.Join(dir, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "ship.md"), brokenTask)
	write(t, filepath.Join(dir, "notes.yaml"), "title: x\n")
	uri := URI(filepath.Join(dir, "wf.yaml"))
	c, done := start(t)

	init := c.call("initialize", map[string]any{"capabilities": map[string]any{}})
	if !strings.Contains(string(init["result"]), `"definitionProvider":true`) {
		t.Fatalf("initialize: got %s", init["result"])
	}
	c.send("initialized", map[string]any{})

	broken := strings.Replace(validWorkflow, "plan: ship", "plan: nowhere", 1)
	c.send("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "yaml", "version": 1, "text": broken}})
	got := c.diagnostics()
	want := []Range{span(6, 8, 15), span(12, 10, 17)}
	if got.URI != uri || len(got.Diagnostics) != len(want) {
		t.Fatalf("didOpen: got %+v, want %d diagnostics on %s", got, len(want), uri)
	}
	for i, d := range got.Diagnostics {
		if d.Range != want[i] || d.Severity != severityError {
			t.Errorf("diagnostic %d: got %+v, want %+v", i, d, want[i])
		}
	}

	write(t, filepath.Join(dir, "ship.md"), validTask)
	c.send("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": validWorkflow}}})
	if got := c.diagnostics(); len(got.Diagnostics) != 0 {
		t.Fatalf("didChange: got %+v, want none", got.Diagnostics)
	}

	var rendered RenderResult
	resp := c.call(MethodRender, RenderParams{URI: uri})
	if err := json.Unmarshal(resp["result"], &rendered); err != nil || !strings.Contains(rendered.HTML, "<html") {
		t.Fatalf("render: got %s", resp)
	}

	c.send("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 3}, "contentChanges": []any{map[string]any{"text": broken}}})
	c.diagnostics()
	resp = c.call(MethodRender, RenderParams{URI: uri})
	if err := json.Unmarshal(resp["result"], &rendered); err != nil || !strings.Contains(rendered.HTML, "nowhere") || resp["error"] != nil {
		t.Fatalf("render invalid: got %s", resp)
	}

	if resp := c.call(MethodRender, RenderParams{URI: URI(filepath.Join(dir, "notes.yaml"))}); resp["error"] == nil {
		t.Fatalf("render other: got %s, want an error", resp)
	}

	resp = c.call("textDocument/definition", positionParams{TextDocument: textDocumentIdentifier{URI: uri}, Position: Position{Line: 12, Character: 12}})
	var locs []Location
	if err := json.Unmarshal(resp["result"], &locs); err != nil || len(locs) != 1 || locs[0].URI != URI(canonical(filepath.Join(dir, "ship.md"))) {
		t.Fatalf("definition: got %s", resp)
	}

	if resp := c.call("nope", nil); !strings.Contains(string(resp["error"]), "-32601") {
		t.Fatalf("unknown method: got %s", resp)
	}

	c.call("shutdown", nil)
	c.send("exit", nil)
	if err := <-done; err != nil {
		t.Fatalf("Serve: %v", err)
	}
}
