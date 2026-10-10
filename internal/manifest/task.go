package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

const fence = "---\n"

var (
	ErrNoFrontmatter   = errors.New("no --- frontmatter fence")
	ErrOpenFrontmatter = errors.New("frontmatter is never closed")
	ErrReservedBody    = errors.New("reserved: the body is the markdown under the frontmatter")
)

func SplitTask(data []byte) ([]byte, string, error) {
	rest, ok := bytes.CutPrefix(data, []byte(fence))
	if !ok {
		return nil, "", ErrNoFrontmatter
	}
	front, body, ok := bytes.Cut(rest, []byte("\n"+fence))
	if !ok {
		return nil, "", ErrOpenFrontmatter
	}
	return data[:len(fence)+len(front)], string(bytes.TrimLeft(body, "\n")), nil
}

var (
	declaration = regexp.MustCompile(`(?m)^["']?openawf["']?[ \t]*:(\s|$)`)
	documentEnd = regexp.MustCompile(`(?m)^(---|\.\.\.)\s*$`)
)

func Declares(data []byte) bool {
	data = bytes.TrimPrefix(data, []byte(fence))
	if end := documentEnd.FindIndex(data); end != nil {
		data = data[:end[0]]
	}
	return declaration.Match(data)
}

func TaskDocument(front []byte, body string) ([]byte, error) {
	raw, err := yaml.YAMLToJSON(front)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return raw, nil
	}
	if _, ok := fields["body"]; ok {
		return nil, fmt.Errorf("body: %w", ErrReservedBody)
	}
	if fields["body"], err = json.Marshal(body); err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func UnmarshalTask(data []byte) (*Task, error) {
	front, body, err := SplitTask(data)
	if err != nil {
		return nil, err
	}
	return DecodeTask(front, body)
}

func DecodeTask(front []byte, body string) (*Task, error) {
	var t Task
	if err := yaml.Unmarshal(front, &t); err != nil {
		return nil, err
	}
	t.Body = Body(body)
	return &t, nil
}

func MarshalTask(t *Task) ([]byte, error) {
	front := *t
	front.SHA, front.Body = nil, ""
	fields, err := yaml.MarshalWithOptions(front, yaml.IndentSequence(true))
	if err != nil {
		return nil, err
	}
	sha, err := yaml.Marshal(map[string]any{"sha": t.SHA})
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString(fence)
	b.Write(fields)
	b.Write(sha)
	b.WriteString(fence)
	b.WriteString("\n")
	b.WriteString(string(t.Body))
	return b.Bytes(), nil
}

type Body string

func (b Body) MarshalYAML() (any, error) {
	first := strings.TrimLeft(string(b), "\n")
	if strings.HasPrefix(first, " ") || strings.HasPrefix(first, "\t") {
		quoted, err := json.Marshal(string(b))
		return yaml.RawMessage(quoted), err
	}
	return string(b), nil
}
