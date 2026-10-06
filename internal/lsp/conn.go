package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"sync"
)

var ErrNoLength = errors.New("no Content-Length header")

type conn struct {
	in  *bufio.Reader
	mu  sync.Mutex
	out io.Writer
}

func newConn(in io.Reader, out io.Writer) *conn {
	return &conn{in: bufio.NewReader(in), out: out}
}

func (c *conn) read() ([]byte, error) {
	header, err := textproto.NewReader(c.in).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	raw := header.Get("Content-Length")
	if raw == "" {
		return nil, ErrNoLength
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("Content-Length %q: %w", raw, ErrNoLength)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.in, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (c *conn) write(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.out.Write(body)
	return err
}
