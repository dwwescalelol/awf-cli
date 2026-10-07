package lsp

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/dwwescalelol/awf-cli/internal/page"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

const methodRender = "awf/render"

var ErrNoShutdown = errors.New("exit without shutdown")

type server struct {
	conn        *conn
	docs        map[string]*document
	initialized bool
	shutdown    bool
}

type document struct {
	uri     string
	path    string
	text    []byte
	version int
	sites   []site
}

func Serve(in io.Reader, out io.Writer) error {
	s := &server{conn: &conn{in: bufio.NewReader(in), out: out}, docs: map[string]*document{}}
	for {
		body, err := s.conn.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			if err := s.reply(nil, nil, &responseError{Code: codeParse, Message: err.Error()}); err != nil {
				return err
			}
			continue
		}
		if m.Method == "exit" {
			if !s.shutdown {
				return ErrNoShutdown
			}
			return nil
		}
		if err := s.handle(m); err != nil {
			return err
		}
	}
}

func (s *server) handle(m message) error {
	switch {
	case m.Method == "":
		return nil
	case m.ID == nil && s.initialized && !s.shutdown:
		return s.notify(m.Method, m.Params)
	case m.ID == nil:
		return nil
	}
	result, err := s.request(m.Method, m.Params)
	return s.reply(m.ID, result, err)
}

func (s *server) reply(id json.RawMessage, result any, err error) error {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	var rerr *responseError
	switch {
	case errors.As(err, &rerr):
		msg["error"] = rerr
	case err != nil:
		msg["error"] = &responseError{Code: codeRequestFailed, Message: err.Error()}
	default:
		msg["result"] = result
	}
	return s.conn.write(msg)
}

var requests = map[string]func(*document, params) (any, error){
	"textDocument/definition": definition,
	"textDocument/codeLens":   codeLens,
	methodRender:              preview,
}

func (s *server) request(method string, raw json.RawMessage) (any, error) {
	switch {
	case s.shutdown:
		return nil, &responseError{Code: codeInvalidRequest, Message: "shut down"}
	case method == "initialize":
		s.initialized = true
		return map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync":   map[string]any{"openClose": true, "change": 1, "save": true},
				"definitionProvider": true,
				"codeLensProvider":   map[string]any{},
			},
			"serverInfo": map[string]string{"name": "awf", "version": build.Version},
		}, nil
	case !s.initialized:
		return nil, &responseError{Code: codeNotInitialized, Message: "not initialized"}
	case method == "shutdown":
		s.shutdown = true
		return nil, nil
	}
	handler, ok := requests[method]
	if !ok {
		return nil, &responseError{Code: codeMethodNotFound, Message: "method not found: " + method}
	}
	var p params
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &responseError{Code: codeInvalidParams, Message: err.Error()}
	}
	d, err := s.document(cmp.Or(p.URI, p.TextDocument.URI))
	if err != nil {
		return nil, err
	}
	return handler(d, p)
}

func (s *server) document(uri string) (*document, error) {
	path, err := filePath(uri)
	if err != nil {
		return nil, &responseError{Code: codeInvalidParams, Message: err.Error()}
	}
	if d, ok := s.docs[uri]; ok {
		return d, nil
	}
	text, err := os.ReadFile(path)
	return &document{uri: uri, path: path, text: text}, err
}

func (s *server) notify(method string, raw json.RawMessage) error {
	var p params
	if json.Unmarshal(raw, &p) != nil {
		return nil
	}
	uri := p.TextDocument.URI
	path, err := filePath(uri)
	if err != nil {
		return nil
	}
	d := &document{uri: uri, path: path, text: []byte(p.TextDocument.Text), version: p.TextDocument.Version}
	switch method {
	case "textDocument/didOpen":
		return s.update(d)
	case "textDocument/didChange":
		if n := len(p.ContentChanges); n > 0 {
			d.text = []byte(p.ContentChanges[n-1].Text)
			return s.update(d)
		}
	case "textDocument/didSave":
		if d, ok := s.docs[uri]; ok {
			return s.update(d)
		}
	case "textDocument/didClose":
		delete(s.docs, uri)
		return s.publish(d, nil)
	}
	return nil
}

func (s *server) update(d *document) error {
	s.docs[d.uri] = d
	key := canonical(d.path)
	for _, other := range s.docs {
		if other != d && !slices.ContainsFunc(other.sites, func(s site) bool { return s.key == key }) {
			continue
		}
		if err := s.publish(other, diagnose(other)); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) publish(d *document, diagnostics []diagnostic) error {
	return s.conn.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params":  publishParams{URI: d.uri, Version: d.version, Diagnostics: append([]diagnostic{}, diagnostics...)},
	})
}

func definition(d *document, p params) (any, error) {
	if kind, err := detect(d.path, d.text); err != nil || kind != store.Workflow {
		return nil, nil
	}
	_, sites, _ := workflow(d.path, d.text)
	for _, site := range sites {
		if site.rng.contains(p.Position) {
			return []location{{URI: fileURI(site.target)}}, nil
		}
	}
	return nil, nil
}

func codeLens(d *document, _ params) (any, error) {
	if _, err := detect(d.path, d.text); err != nil {
		return nil, nil
	}
	command := map[string]any{"title": "Open Preview", "command": "awf.preview", "arguments": []string{d.uri}}
	return []map[string]any{{"range": textRange{}, "command": command}}, nil
}

func preview(d *document, _ params) (any, error) {
	kind, err := detect(d.path, d.text)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = page.Document(&out, kind, d.path, d.text, scope(d.path), nil, render.Scope{}, "")
	if err != nil && !errors.Is(err, page.ErrInvalid) {
		return nil, err
	}
	return map[string]string{"html": out.String()}, nil
}
