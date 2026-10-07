package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
)

var ErrNoLength = errors.New("no Content-Length header")

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeNotInitialized = -32002
	codeRequestFailed  = -32803
)

type conn struct {
	in  *bufio.Reader
	out io.Writer
}

func (c *conn) read() ([]byte, error) {
	header, err := textproto.NewReader(c.in).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(header.Get("Content-Length"))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Content-Length %q: %w", header.Get("Content-Length"), ErrNoLength)
	}
	body := make([]byte, n)
	_, err = io.ReadFull(c.in, body)
	return body, err
}

func (c *conn) write(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type params struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
		Text    string `json:"text"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
	Position position `json:"position"`
	URI      string   `json:"uri"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *responseError) Error() string { return e.Message }

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type textRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

func (r textRange) contains(p position) bool {
	return p.Line == r.Start.Line && p.Character >= r.Start.Character && p.Character <= r.End.Character
}

type location struct {
	URI   string    `json:"uri"`
	Range textRange `json:"range"`
}

type diagnostic struct {
	Range    textRange `json:"range"`
	Severity int       `json:"severity"`
	Source   string    `json:"source"`
	Message  string    `json:"message"`
	Related  []related `json:"relatedInformation,omitempty"`
}

type related struct {
	Location location `json:"location"`
	Message  string   `json:"message"`
}

type publishParams struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []diagnostic `json:"diagnostics"`
}
