// Package manifest reads and writes OpenAWF workflow documents.
package manifest

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

var (
	ErrNotAnEdge  = errors.New("not a task name or an edge")
	ErrNotATask   = errors.New("not a task or a $ref")
	ErrNotTools   = errors.New(`not a tool list or "*"`)
	ErrNotModel   = errors.New("not a model or a list of models")
	ErrNotSchema  = errors.New("not a JSON Schema object")
	ErrNotOutcome = errors.New("not an outcome name")
)

type Workflow struct {
	OpenAWF       version.Version      `yaml:"openawf"`
	Name          string               `yaml:"name"`
	Summary       string               `yaml:"summary,omitempty"`
	Model         Models               `yaml:"model,omitempty"`
	Version       version.Version      `yaml:"version,omitempty"`
	SHA           *string              `yaml:"sha"`
	Source        string               `yaml:"source,omitempty"`
	Start         string               `yaml:"start"`
	Orchestration Orchestration        `yaml:"orchestration"`
	Tasks         Tasks                `yaml:"tasks"`
	MCP           map[string]MCPServer `yaml:"mcp,omitempty"`
	Rest          Fields               `yaml:",inline"`
}

// Fields holds the members of an object that have no field of their own, such
// as extension keys, so reading and writing a document keeps them.
type Fields map[string]any

// Models is one model, or a fallback chain of them.
type Models []string

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
	Rest    Fields `yaml:",inline"`
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
	Model    Models          `yaml:"model,omitempty"`
	Outcomes Outcomes        `yaml:"outcomes,omitempty"`
	Uses     []string        `yaml:"uses,omitempty"`
	Body     Body            `yaml:"body,omitempty"`
	Rest     Fields          `yaml:",inline"`
}

// Outcomes maps each outcome a task may report to the JSON Schema of its
// result, in declaration order.
type Outcomes []Outcome

type Outcome struct {
	Name   string
	Schema map[string]any
}

func (o Outcomes) Names() []string {
	names := make([]string, len(o))
	for i, outcome := range o {
		names[i] = outcome.Name
	}
	return names
}

type MCPServer struct {
	Type  string    `yaml:"type"`
	Allow *MCPTools `yaml:"allow,omitempty"`
	Deny  []string  `yaml:"deny,omitempty"`
	Rest  Fields    `yaml:",inline"`
}

type MCPTools struct {
	All   bool
	Names []string
}

// UnmarshalYAML reads the workflow's own fields and keeps every other member
// in Rest.
func (w *Workflow) UnmarshalYAML(data []byte) error {
	type workflow Workflow
	var fields workflow
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*w = Workflow(fields)
	w.Rest = w.Rest.without(w)
	return nil
}

func (t *Task) UnmarshalYAML(data []byte) error {
	type task Task
	var fields task
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*t = Task(fields)
	t.Rest = t.Rest.without(t)
	return nil
}

func (s *MCPServer) UnmarshalYAML(data []byte) error {
	type server MCPServer
	var fields server
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*s = MCPServer(fields)
	s.Rest = s.Rest.without(s)
	return nil
}

// without drops the members that the struct v points at holds in fields of
// its own. An inline map receives every member, not only the unknown ones.
func (f Fields) without(v any) Fields {
	t := reflect.TypeOf(v).Elem()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		delete(f, name)
	}
	if len(f) == 0 {
		return nil
	}
	return f
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
		e.Rest = e.Rest.without(e)
	default:
		return fmt.Errorf("edge %T: %w", node, ErrNotAnEdge)
	}
	return nil
}

func (e Edge) MarshalYAML() (any, error) {
	if e.Retries == nil && e.Session == "" && len(e.Rest) == 0 {
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
			return fmt.Errorf("allow %q: %w", node, ErrNotTools)
		}
		*t = MCPTools{All: true}
	case []any:
		var names []string
		if err := yaml.Unmarshal(data, &names); err != nil {
			return err
		}
		*t = MCPTools{Names: names}
	default:
		return fmt.Errorf("allow %T: %w", node, ErrNotTools)
	}
	return nil
}

func (t MCPTools) MarshalYAML() (any, error) {
	if t.All {
		return "*", nil
	}
	return t.Names, nil
}

// String writes the chain in order, separated by commas.
func (m Models) String() string {
	return strings.Join(m, ", ")
}

func (m *Models) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}
	switch node := value.(type) {
	case string:
		*m = Models{node}
	case []any:
		var names []string
		if err := yaml.Unmarshal(data, &names); err != nil {
			return err
		}
		*m = names
	default:
		return fmt.Errorf("model %T: %w", node, ErrNotModel)
	}
	return nil
}

func (m Models) MarshalYAML() (any, error) {
	if len(m) == 1 {
		return m[0], nil
	}
	return []string(m), nil
}

func (o *Outcomes) UnmarshalYAML(data []byte) error {
	var items yaml.MapSlice
	if err := yaml.Unmarshal(data, &items); err != nil {
		return err
	}
	out := make(Outcomes, 0, len(items))
	for _, item := range items {
		name, ok := item.Key.(string)
		if !ok {
			return fmt.Errorf("outcome %v: %w", item.Key, ErrNotOutcome)
		}
		schema, ok := item.Value.(map[string]any)
		if !ok {
			return fmt.Errorf("outcome %s %T: %w", name, item.Value, ErrNotSchema)
		}
		out = append(out, Outcome{Name: name, Schema: schema})
	}
	*o = out
	return nil
}

func (o Outcomes) MarshalYAML() (any, error) {
	items := make(yaml.MapSlice, 0, len(o))
	for _, outcome := range o {
		schema := outcome.Schema
		if schema == nil {
			schema = map[string]any{}
		}
		items = append(items, yaml.MapItem{Key: outcome.Name, Value: schema})
	}
	return items, nil
}
