package lsp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/page"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

const MethodRender = "awf/render"

var (
	ErrNoShutdown = errors.New("exit without shutdown")
	ErrNotFileURI = errors.New("not a file uri")
)

type server struct {
	conn      *conn
	docs      map[string][]byte
	published map[string]bool
	shutdown  bool
}

func Serve(in io.Reader, out io.Writer) error {
	s := &server{conn: newConn(in, out), docs: map[string][]byte{}, published: map[string]bool{}}
	for {
		body, err := s.conn.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		done, err := s.handle(body)
		if done || err != nil {
			return err
		}
	}
}

func (s *server) handle(body []byte) (bool, error) {
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return false, s.reply(nil, nil, &responseError{Code: codeParse, Message: err.Error()})
	}
	if m.Method == "exit" {
		if !s.shutdown {
			return true, ErrNoShutdown
		}
		return true, nil
	}
	if m.ID == nil {
		s.notify(m.Method, m.Params)
		return false, nil
	}
	result, err := s.request(m.Method, m.Params)
	var rerr *responseError
	if err != nil && !errors.As(err, &rerr) {
		rerr = &responseError{Code: codeRequestFailed, Message: err.Error()}
	}
	return false, s.reply(m.ID, result, rerr)
}

func (s *server) reply(id *json.RawMessage, result any, err *responseError) error {
	if id == nil {
		null := json.RawMessage("null")
		id = &null
	}
	if err != nil {
		return s.conn.write(errorResponse{JSONRPC: "2.0", ID: id, Error: err})
	}
	return s.conn.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *server) request(method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return initializeResult{
			Capabilities: capabilities{
				TextDocumentSync:   textDocumentSync{OpenClose: true, Change: syncFull, Save: saveOptions{IncludeText: true}},
				DefinitionProvider: true,
			},
			ServerInfo: serverInfo{Name: "awf", Version: build.Version},
		}, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "textDocument/definition":
		var p positionParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.definition(p)
	case MethodRender:
		var p RenderParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		return s.render(p.URI)
	}
	return nil, &responseError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func decode(params json.RawMessage, v any) error {
	if err := json.Unmarshal(params, v); err != nil {
		return &responseError{Code: codeInvalidParams, Message: err.Error()}
	}
	return nil
}

func (s *server) notify(method string, params json.RawMessage) {
	switch method {
	case "textDocument/didOpen":
		var p didOpenParams
		if json.Unmarshal(params, &p) == nil {
			s.update(p.TextDocument.URI, []byte(p.TextDocument.Text))
		}
	case "textDocument/didChange":
		var p didChangeParams
		if json.Unmarshal(params, &p) == nil && len(p.ContentChanges) > 0 {
			s.update(p.TextDocument.URI, []byte(p.ContentChanges[len(p.ContentChanges)-1].Text))
		}
	case "textDocument/didSave":
		var p didSaveParams
		if json.Unmarshal(params, &p) == nil {
			s.save(p)
		}
	case "textDocument/didClose":
		var p didCloseParams
		if json.Unmarshal(params, &p) == nil {
			delete(s.docs, p.TextDocument.URI)
			s.publish(p.TextDocument.URI, nil)
		}
	}
}

func (s *server) save(p didSaveParams) {
	if p.Text != nil {
		s.update(p.TextDocument.URI, []byte(*p.Text))
		return
	}
	if text, ok := s.docs[p.TextDocument.URI]; ok {
		s.update(p.TextDocument.URI, text)
	}
}

func (s *server) update(uri string, text []byte) {
	s.docs[uri] = text
	path, err := Path(uri)
	if err != nil {
		s.publish(uri, nil)
		return
	}
	s.publish(uri, Check(path, text))
}

func (s *server) publish(uri string, diagnostics []Diagnostic) {
	if len(diagnostics) == 0 && !s.published[uri] {
		return
	}
	s.published[uri] = len(diagnostics) > 0
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	s.conn.write(notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: publishParams{URI: uri, Diagnostics: diagnostics}})
}

func (s *server) text(uri string) (string, []byte, error) {
	path, err := Path(uri)
	if err != nil {
		return "", nil, err
	}
	if text, ok := s.docs[uri]; ok {
		return path, text, nil
	}
	text, err := os.ReadFile(path)
	return path, text, err
}

func (s *server) render(uri string) (RenderResult, error) {
	path, text, err := s.text(uri)
	if err != nil {
		return RenderResult{}, err
	}
	html, err := Render(path, text)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{HTML: string(html)}, nil
}

func (s *server) definition(p positionParams) ([]Location, error) {
	path, text, err := s.text(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	if kind, err := Detect(path, text); err != nil || kind != store.Workflow {
		return []Location{}, nil
	}
	for _, site := range refSites(path, text, scope(path)) {
		if site.rng.contains(p.Position) {
			return []Location{{URI: URI(site.target)}}, nil
		}
	}
	return []Location{}, nil
}

func Check(path string, text []byte) []Diagnostic {
	kind, err := Detect(path, text)
	if err != nil {
		return nil
	}
	if kind == store.Task {
		_, err := load.ReadTaskData(path, text, nil)
		return diagnose(path, text, problemsOf(path, err, nil), nil)
	}
	s := scope(path)
	f, err := load.ReadWorkflowData(path, text, s, nil)
	return diagnose(path, text, problemsOf(path, err, f.Warnings), refSites(path, text, s))
}

func Render(path string, text []byte) ([]byte, error) {
	kind, err := Detect(path, text)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if kind == store.Task {
		err = page.TaskData(&out, path, text, nil)
	} else {
		err = page.WorkflowData(&out, path, text, scope(path), nil, render.Scope{}, "")
	}
	if err != nil && !errors.Is(err, page.ErrInvalid) {
		return nil, err
	}
	return out.Bytes(), nil
}

func scope(path string) *store.Store {
	dir, err := store.FindDirFrom(filepath.Dir(canonical(path)))
	if err != nil {
		return nil
	}
	return store.New(dir)
}

func Path(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("%s: %w", uri, ErrNotFileURI)
	}
	return filepath.FromSlash(u.Path), nil
}

func URI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
