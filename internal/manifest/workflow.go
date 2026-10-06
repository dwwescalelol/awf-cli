// Package manifest reads and writes OpenAWF workflow documents.
package manifest

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

var (
	ErrNotAnEdge = errors.New("not a task name or an edge")
	ErrNotATask  = errors.New("not a task or a $ref")
	ErrNotTools  = errors.New(`not a tool list or "*"`)
)

type Workflow struct {
	OpenAWF       version.Version      `yaml:"openawf"`
	Name          string               `yaml:"name"`
	Summary       string               `yaml:"summary,omitempty"`
	Model         string               `yaml:"model,omitempty"`
	Version       version.Version      `yaml:"version,omitempty"`
	SHA           *string              `yaml:"sha"`
	Source        string               `yaml:"source,omitempty"`
	Start         string               `yaml:"start"`
	Orchestration Orchestration        `yaml:"orchestration"`
	Tasks         Tasks                `yaml:"tasks"`
	MCP           map[string]MCPServer `yaml:"mcp,omitempty"`
}

type Orchestration map[string]Transition

type Transition struct {
	Edge   *Edge
	Branch Branch
}

type Branch map[string]Edge

type Edge struct {
	Task    string `yaml:"task"`
	Retries *int   `yaml:"retries,omitempty"`
	Session string `yaml:"session,omitempty"`
}

type Tasks map[string]TaskEntry

type TaskEntry struct {
	Task *Task
	Ref  string
}

type Task struct {
	OpenAWF  version.Version `yaml:"openawf,omitempty"`
	Version  version.Version `yaml:"version,omitempty"`
	SHA      *string         `yaml:"sha,omitempty"`
	Source   string          `yaml:"source,omitempty"`
	Summary  string          `yaml:"summary,omitempty"`
	Input    map[string]any  `yaml:"input,omitempty"`
	Output   map[string]any  `yaml:"output,omitempty"`
	Model    string          `yaml:"model,omitempty"`
	Outcomes []string        `yaml:"outcomes,omitempty"`
	Uses     []string        `yaml:"uses,omitempty"`
	Body     Body            `yaml:"body,omitempty"`
}

type MCPServer struct {
	Transport string    `yaml:"transport,omitempty"`
	Tools     *MCPTools `yaml:"tools,omitempty"`
}

type MCPTools struct {
	All   bool
	Names []string
}

func Unmarshal(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

func Marshal(wf *Workflow) ([]byte, error) {
	return yaml.MarshalWithOptions(wf, yaml.UseLiteralStyleIfMultiline(true), yaml.IndentSequence(true))
}

func (t *Transition) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	outcomes, isMapping := value.(map[string]any)
	_, isEdge := outcomes["task"]
	switch {
	case value == nil:
		*t = Transition{}
	case isMapping && !isEdge:
		var branch Branch
		if err := yaml.Unmarshal(data, &branch); err != nil {
			return err
		}
		*t = Transition{Branch: branch}
	default:
		var edge Edge
		if err := edge.UnmarshalYAML(data); err != nil {
			return err
		}
		*t = Transition{Edge: &edge}
	}
	return nil
}

func (t Transition) MarshalYAML() (any, error) {
	switch {
	case t.Edge != nil:
		return *t.Edge, nil
	case t.Branch != nil:
		return t.Branch, nil
	}
	return nil, nil
}

func (e *Edge) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	switch node := value.(type) {
	case string:
		*e = Edge{Task: node}
	case map[string]any:
		type edge Edge
		var fields edge
		if err := yaml.Unmarshal(data, &fields); err != nil {
			return err
		}
		*e = Edge(fields)
	default:
		return fmt.Errorf("edge %T: %w", node, ErrNotAnEdge)
	}
	return nil
}

func (e Edge) MarshalYAML() (any, error) {
	if e.Retries == nil && e.Session == "" {
		return e.Task, nil
	}
	type edge Edge
	return edge(e), nil
}

func (e *TaskEntry) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	fields, isMapping := value.(map[string]any)
	if !isMapping {
		return fmt.Errorf("task %T: %w", value, ErrNotATask)
	}
	if _, isRef := fields["$ref"]; isRef {
		var ref struct {
			Ref string `yaml:"$ref"`
		}
		if err := yaml.Unmarshal(data, &ref); err != nil {
			return err
		}
		*e = TaskEntry{Ref: ref.Ref}
		return nil
	}
	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return err
	}
	*e = TaskEntry{Task: &task}
	return nil
}

func (e TaskEntry) MarshalYAML() (any, error) {
	if e.Ref != "" {
		return map[string]string{"$ref": e.Ref}, nil
	}
	return e.Task, nil
}

func (t *MCPTools) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	switch node := value.(type) {
	case string:
		if node != "*" {
			return fmt.Errorf("tools %q: %w", node, ErrNotTools)
		}
		*t = MCPTools{All: true}
	case []any:
		var names []string
		if err := yaml.Unmarshal(data, &names); err != nil {
			return err
		}
		*t = MCPTools{Names: names}
	default:
		return fmt.Errorf("tools %T: %w", node, ErrNotTools)
	}
	return nil
}

func (t MCPTools) MarshalYAML() (any, error) {
	if t.All {
		return "*", nil
	}
	return t.Names, nil
}
