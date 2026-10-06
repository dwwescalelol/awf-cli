package lsp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

var ErrNotOpenAWF = errors.New("not an OpenAWF document")

var versionKey = regexp.MustCompile(`^["']?openawf["']?\s*:`)

func Detect(path string, text []byte) (store.DocumentKind, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if hasVersionKey(text) {
			return store.Workflow, nil
		}
	case ".md":
		if front, ok := frontmatter(text); ok && hasVersionKey(front) {
			return store.Task, nil
		}
	}
	return store.DocumentKind{}, fmt.Errorf("%s: %w", path, ErrNotOpenAWF)
}

func hasVersionKey(text []byte) bool {
	lines := bufio.NewScanner(bytes.NewReader(text))
	for lines.Scan() {
		if versionKey.Match(lines.Bytes()) {
			return true
		}
	}
	return false
}

func frontmatter(text []byte) ([]byte, bool) {
	text = bytes.ReplaceAll(text, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(text, []byte("---\n"))
	if !ok {
		return nil, false
	}
	front, _, ok := bytes.Cut(rest, []byte("\n---"))
	if !ok {
		return rest, true
	}
	return front, true
}
