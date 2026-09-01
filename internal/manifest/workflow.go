// Package manifest reads and writes OpenAWF workflow documents.
package manifest

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

type Workflow struct {
	OpenAWF       Version              `yaml:"openawf"`
	Name          string               `yaml:"name"`
	Summary       string               `yaml:"summary,omitempty"`
	Model         string               `yaml:"model,omitempty"`
	Version       Version              `yaml:"version,omitempty"`
	SHA           *string              `yaml:"sha,omitempty"`
	Source        string               `yaml:"source,omitempty"`
	Start         string               `yaml:"start"`
	Orchestration Orchestration        `yaml:"orchestration"`
	Tasks         Tasks                `yaml:"tasks"`
	MCP           map[string]MCPServer `yaml:"mcp,omitempty"`
	XMeta         map[string]any       `yaml:"x-meta,omitempty"`
}

// Version keeps the version exactly as authored. YAML infers `1.0` as a float,
// and formatting that back gives "1", so a numeric scalar is taken as source
// text rather than converted.
type Version string

func (v *Version) UnmarshalYAML(node ast.Node) error {
	if s, ok := node.(*ast.StringNode); ok {
		*v = Version(s.Value)
		return nil
	}
	*v = Version(node.String())
	return nil
}

func Parse(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := decode(data, &wf); err != nil {
		var pe *ParseError
		if errors.As(err, &pe) {
			return nil, pe
		}
		return nil, &ParseError{Err: err}
	}
	return &wf, nil
}

func Marshal(wf *Workflow) ([]byte, error) {
	return yaml.MarshalWithOptions(wf, yaml.UseLiteralStyleIfMultiline(true))
}

// rawNode defers decoding so a map can report the key a failing value sits under.
type rawNode []byte

func (r *rawNode) UnmarshalYAML(data []byte) error {
	*r = data
	return nil
}

// go-yaml does not call UnmarshalYAML for a null node, so a null value leaves
// the rawNode empty; the unions still have to see it.
func (r rawNode) bytes() []byte {
	if len(r) == 0 {
		return []byte("null")
	}
	return r
}

func rawMap(data []byte) (map[string]rawNode, error) {
	var m map[string]rawNode
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// decode rejects unknown fields (the schema sets additionalProperties: false)
// and keeps a ParseError raised further down intact.
func decode(data []byte, v any) error {
	err := yaml.UnmarshalWithOptions(data, v, yaml.DisallowUnknownField())
	var pe *ParseError
	if errors.As(err, &pe) {
		return pe
	}
	return err
}

// Orchestration maps a task name to the transition out of it.
type Orchestration map[string]Transition

func (o *Orchestration) UnmarshalYAML(data []byte) error {
	m, err := rawMap(data)
	if err != nil {
		return wrap("orchestration", err)
	}
	out := make(Orchestration, len(m))
	for name, raw := range m {
		var t Transition
		if err := decode(raw.bytes(), &t); err != nil {
			return wrap("orchestration", wrap(name, err))
		}
		out[name] = t
	}
	*o = out
	return nil
}

// Transition is where a task goes next: an edge, a branch, or neither, which
// ends the flow. Terminal is derived because go-yaml never calls
// UnmarshalYAML for the null node that spells it.
type Transition struct {
	Edge   *Edge
	Branch Branch
}

func (t Transition) Terminal() bool { return t.Edge == nil && t.Branch == nil }

func (t *Transition) UnmarshalYAML(data []byte) error {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	switch node := v.(type) {
	case nil:
		*t = Transition{}
		return nil
	case map[string]any:
		// "task" is reserved as the marker of an edge object, so an object
		// without it is a branch.
		if _, ok := node["task"]; !ok {
			var b Branch
			if err := decode(data, &b); err != nil {
				return err
			}
			*t = Transition{Branch: b}
			return nil
		}
	}
	var e Edge
	if err := decode(data, &e); err != nil {
		return err
	}
	*t = Transition{Edge: &e}
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

// Branch maps each outcome a task emits to the edge it takes.
type Branch map[string]Edge

func (b *Branch) UnmarshalYAML(data []byte) error {
	m, err := rawMap(data)
	if err != nil {
		return err
	}
	out := make(Branch, len(m))
	for outcome, raw := range m {
		var e Edge
		if err := decode(raw.bytes(), &e); err != nil {
			return wrap(outcome, err)
		}
		out[outcome] = e
	}
	*b = out
	return nil
}

type Edge struct {
	Task    string `yaml:"task"`
	Retries *int   `yaml:"retries,omitempty"`
	Session string `yaml:"session,omitempty"`
}

func (e *Edge) UnmarshalYAML(data []byte) error {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	switch node := v.(type) {
	case nil:
		return errors.New("edge: null is not a task")
	case string:
		*e = Edge{Task: node}
		return nil
	case map[string]any:
		type edge Edge
		var out edge
		if err := decode(data, &out); err != nil {
			return err
		}
		*e = Edge(out)
		return nil
	}
	return fmt.Errorf("edge: %T is not a task name or an edge", v)
}

func (e Edge) MarshalYAML() (any, error) {
	if e.Retries == nil && e.Session == "" {
		return e.Task, nil
	}
	type edge Edge
	return edge(e), nil
}

// Tasks maps a task name to its definition or to a reference to one.
type Tasks map[string]TaskEntry

func (t *Tasks) UnmarshalYAML(data []byte) error {
	m, err := rawMap(data)
	if err != nil {
		return wrap("tasks", err)
	}
	out := make(Tasks, len(m))
	for name, raw := range m {
		var e TaskEntry
		if err := decode(raw.bytes(), &e); err != nil {
			return wrap("tasks", wrap(name, err))
		}
		out[name] = e
	}
	*t = out
	return nil
}

// TaskEntry is an inline task or a $ref to one defined elsewhere.
type TaskEntry struct {
	Task *Task
	Ref  string
}

func (e *TaskEntry) UnmarshalYAML(data []byte) error {
	var fields map[string]any
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["$ref"]; ok {
		var r struct {
			Ref string `yaml:"$ref"`
		}
		if err := decode(data, &r); err != nil {
			return err
		}
		*e = TaskEntry{Ref: r.Ref}
		return nil
	}
	var task Task
	if err := decode(data, &task); err != nil {
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

type Task struct {
	Version  Version        `yaml:"version,omitempty"`
	SHA      *string        `yaml:"sha,omitempty"`
	Source   string         `yaml:"source,omitempty"`
	Summary  string         `yaml:"summary,omitempty"`
	Input    map[string]any `yaml:"input,omitempty"`
	Output   map[string]any `yaml:"output,omitempty"`
	Model    string         `yaml:"model,omitempty"`
	Outcomes []string       `yaml:"outcomes,omitempty"`
	Uses     []string       `yaml:"uses,omitempty"`
	Body     string         `yaml:"body"`
	XMeta    map[string]any `yaml:"x-meta,omitempty"`
}

type MCPServer struct {
	Transport string    `yaml:"transport,omitempty"`
	Tools     *MCPTools `yaml:"tools,omitempty"`
}

// MCPTools is the set of tools a workflow uses from a server: every tool, or
// a named list.
type MCPTools struct {
	All   bool
	Names []string
}

func (t *MCPTools) UnmarshalYAML(data []byte) error {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	if s, ok := v.(string); ok {
		if s != "*" {
			return fmt.Errorf("tools: %q is not a tool list or \"*\"", s)
		}
		*t = MCPTools{All: true}
		return nil
	}
	var names []string
	if err := yaml.Unmarshal(data, &names); err != nil {
		return err
	}
	*t = MCPTools{Names: names}
	return nil
}

func (t MCPTools) MarshalYAML() (any, error) {
	if t.All {
		return "*", nil
	}
	return t.Names, nil
}
