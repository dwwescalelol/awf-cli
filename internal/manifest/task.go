package manifest

import (
	"bytes"
	"errors"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

const fence = "---\n"

var (
	ErrNoFrontmatter   = errors.New("no --- frontmatter fence")
	ErrOpenFrontmatter = errors.New("frontmatter is never closed")
)

// UnmarshalTask reads a task file: YAML frontmatter between fences, then the
// markdown body.
func UnmarshalTask(data []byte) (*Task, error) {
	rest, ok := bytes.CutPrefix(data, []byte(fence))
	if !ok {
		return nil, ErrNoFrontmatter
	}
	front, body, ok := bytes.Cut(rest, []byte("\n"+fence))
	if !ok {
		return nil, ErrOpenFrontmatter
	}

	var t Task
	if err := yaml.Unmarshal(front, &t); err != nil {
		return nil, err
	}
	t.Body = string(bytes.TrimLeft(body, "\n"))
	return &t, nil
}

// MarshalTask writes a task file. The body is the markdown under the
// frontmatter, so it is never a YAML field.
func MarshalTask(t *Task) ([]byte, error) {
	data, err := yaml.Marshal(frontmatter{
		Version:  t.Version,
		SHA:      t.SHA,
		Source:   t.Source,
		Summary:  t.Summary,
		Input:    t.Input,
		Output:   t.Output,
		Model:    t.Model,
		Outcomes: t.Outcomes,
		Uses:     t.Uses,
		XMeta:    t.XMeta,
	})
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

// frontmatter is a Task without its body, and with the sha always written,
// since a null sha is what marks a draft.
type frontmatter struct {
	Version  version.Version `yaml:"version,omitempty"`
	SHA      *string         `yaml:"sha"`
	Source   string          `yaml:"source,omitempty"`
	Summary  string          `yaml:"summary,omitempty"`
	Input    map[string]any  `yaml:"input,omitempty"`
	Output   map[string]any  `yaml:"output,omitempty"`
	Model    string          `yaml:"model,omitempty"`
	Outcomes []string        `yaml:"outcomes,omitempty"`
	Uses     []string        `yaml:"uses,omitempty"`
	XMeta    map[string]any  `yaml:"x-meta,omitempty"`
}
