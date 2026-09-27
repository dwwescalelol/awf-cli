package manifest

import (
	"bytes"
	"errors"

	"github.com/goccy/go-yaml"
)

const fence = "---\n"

var (
	ErrNoFrontmatter   = errors.New("no --- frontmatter fence")
	ErrOpenFrontmatter = errors.New("frontmatter is never closed")
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
	return front, string(bytes.TrimLeft(body, "\n")), nil
}

func UnmarshalTask(data []byte) (*Task, error) {
	front, body, err := SplitTask(data)
	if err != nil {
		return nil, err
	}
	var t Task
	if err := yaml.Unmarshal(front, &t); err != nil {
		return nil, err
	}
	t.Body = body
	return &t, nil
}

func MarshalTask(t *Task) ([]byte, error) {
	front := *t
	front.Body = ""
	data, err := yaml.MarshalWithOptions(front, yaml.IndentSequence(true))
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString(fence)
	b.Write(data)
	b.WriteString(fence)
	b.WriteString("\n")
	b.WriteString(t.Body)
	return b.Bytes(), nil
}
