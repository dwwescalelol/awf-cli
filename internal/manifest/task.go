package manifest

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

const fence = "---\n"

// ParseTask reads a task file: YAML frontmatter between fences, then the
// markdown body.
func ParseTask(data []byte) (*Task, error) {
	rest, ok := bytes.CutPrefix(data, []byte(fence))
	if !ok {
		return nil, &ParseError{Err: fmt.Errorf("task: no %q frontmatter fence", "---")}
	}
	front, body, ok := bytes.Cut(rest, []byte("\n"+fence))
	if !ok {
		return nil, &ParseError{Err: fmt.Errorf("task: frontmatter is never closed")}
	}

	var t Task
	if err := decode(front, &t); err != nil {
		var pe *ParseError
		if errors.As(err, &pe) {
			return nil, pe
		}
		return nil, &ParseError{Err: err}
	}
	t.Body = string(bytes.TrimLeft(body, "\n"))
	return &t, nil
}

// MarshalTask writes a task file. The body is the markdown under the
// frontmatter, so it is never a YAML field.
func MarshalTask(t *Task) ([]byte, error) {
	front := *t
	front.Body = ""

	data, err := yaml.Marshal(taskFrontmatter(front))
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

// taskFrontmatter is a Task without its body, and with the sha always written,
// since a null sha is what marks a draft.
type taskFrontmatter struct {
	Version  version.Version `yaml:"version,omitempty"`
	SHA      *string         `yaml:"sha"`
	Source   string          `yaml:"source,omitempty"`
	Summary  string          `yaml:"summary,omitempty"`
	Input    map[string]any  `yaml:"input,omitempty"`
	Output   map[string]any  `yaml:"output,omitempty"`
	Model    string          `yaml:"model,omitempty"`
	Outcomes []string        `yaml:"outcomes,omitempty"`
	Uses     []string        `yaml:"uses,omitempty"`
	Body     string          `yaml:"-"`
	XMeta    map[string]any  `yaml:"x-meta,omitempty"`
}
