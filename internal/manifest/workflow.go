// Package manifest reads and writes OpenAWF workflow documents.
package manifest

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

var (
	ErrNotAnEdge      = errors.New("not a task name or an edge")
	ErrNotATask       = errors.New("not a task or a $ref")
	ErrNotTools       = errors.New(`not a tool list or "*"`)
	ErrNotModel       = errors.New("not a model or a list of models")
	ErrNoModels       = errors.New("an empty list of models")
	ErrDuplicateModel = errors.New("lists a model more than once")
	ErrNotSchema      = errors.New("not a JSON Schema object")
	ErrNotOutcome     = errors.New("not an outcome name")
)

type Workflow struct {
	OpenAWF         version.Version      `yaml:"openawf"`
	Name            string               `yaml:"name"`
	Summary         string               `yaml:"summary,omitempty"`
	Model           Models               `yaml:"model,omitempty"`
	Effort          Effort               `yaml:"effort,omitempty"`
	Metadata        Metadata             `yaml:"metadata,omitempty"`
	License         string               `yaml:"license,omitempty"`
	Version         version.Version      `yaml:"version,omitempty"`
	SHA             *string              `yaml:"sha"`
	ExternalVersion string               `yaml:"external-version,omitempty"`
	Source          string               `yaml:"source,omitempty"`
	Start           string               `yaml:"start"`
	Orchestration   Orchestration        `yaml:"orchestration"`
	Tasks           Tasks                `yaml:"tasks"`
	MCP             map[string]MCPServer `yaml:"mcp,omitempty"`
	Extensions      Extensions           `yaml:"-"`
}

// Extensions holds the members of an object whose names start with x-, in the
// order they are written, with their values as written.
type Extensions yaml.MapSlice

// Value returns the value of the extension key, and whether it is present.
func (e Extensions) Value(key string) (any, bool) {
	return lookup(yaml.MapSlice(e), key)
}

// Metadata holds the free keys that describe a workflow or a task, in the
// order they are written, with their values as written.
type Metadata yaml.MapSlice

func (m Metadata) Value(key string) (any, bool) {
	return lookup(yaml.MapSlice(m), key)
}

// Tags returns the labels for search and grouping.
func (m Metadata) Tags() []string {
	return m.strings("tags")
}

// Authors returns the people or organisations who wrote the document.
func (m Metadata) Authors() []string {
	return m.strings("authors")
}

// Icon returns the URL or relative path of an image that represents the
// document.
func (m Metadata) Icon() string {
	icon, _ := lookup(yaml.MapSlice(m), "icon")
	s, _ := icon.(string)
	return s
}

func (m Metadata) strings(key string) []string {
	value, _ := lookup(yaml.MapSlice(m), key)
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func lookup(items yaml.MapSlice, key string) (any, bool) {
	for _, item := range items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}

// Fields holds the members of a server that have no field of their own.
type Fields map[string]any

// Models is one model, or a fallback chain of distinct models, each written
// provider/name.
type Models []string

// Effort is how much reasoning a model spends on a task.
type Effort string

const (
	Low    Effort = "low"
	Medium Effort = "medium"
	High   Effort = "high"
)

type Orchestration map[string]Transition

type Transition struct {
	Edge   *Edge
	Branch Branch
}

type Branch map[string]Edge

type Edge struct {
	Task       string     `yaml:"task"`
	Retries    *int       `yaml:"retries,omitempty"`
	Session    Session    `yaml:"session,omitempty"`
	Extensions Extensions `yaml:"-"`
}

// Session is how a task starts when an edge leads to it.
type Session string

const (
	// Fresh starts the task in a new agent session.
	Fresh Session = "fresh"
	// Resume continues the target task's own previous session, with the new
	// input as the next message. On the task's first run it starts fresh.
	Resume Session = "resume"
)

type Tasks map[string]TaskEntry

type TaskEntry struct {
	Task *Task
	Ref  string
}

type Task struct {
	OpenAWF         version.Version `yaml:"openawf,omitempty"`
	Version         version.Version `yaml:"version,omitempty"`
	SHA             *string         `yaml:"sha,omitempty"`
	ExternalVersion string          `yaml:"external-version,omitempty"`
	Source          string          `yaml:"source,omitempty"`
	Summary         string          `yaml:"summary,omitempty"`
	Metadata        Metadata        `yaml:"metadata,omitempty"`
	License         string          `yaml:"license,omitempty"`
	Input           map[string]any  `yaml:"input,omitempty"`
	Output          map[string]any  `yaml:"output,omitempty"`
	Model           Models          `yaml:"model,omitempty"`
	Effort          Effort          `yaml:"effort,omitempty"`
	MaxTurns        int             `yaml:"max-turns,omitempty"`
	Timeout         int             `yaml:"timeout,omitempty"`
	Outcomes        Outcomes        `yaml:"outcomes,omitempty"`
	Uses            []string        `yaml:"uses,omitempty"`
	System          Body            `yaml:"system,omitempty"`
	Body            Body            `yaml:"body,omitempty"`
	Extensions      Extensions      `yaml:"-"`
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
	Type       string     `yaml:"type"`
	Allow      *MCPTools  `yaml:"allow,omitempty"`
	Deny       []string   `yaml:"deny,omitempty"`
	Rest       Fields     `yaml:",inline"`
	Extensions Extensions `yaml:"-"`
}

type MCPTools struct {
	All   bool
	Names []string
}

// UnmarshalYAML reads the workflow's fields and its extension keys.
func (w *Workflow) UnmarshalYAML(data []byte) error {
	type workflow Workflow
	var fields workflow
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*w = Workflow(fields)
	var err error
	w.Extensions, err = extensions(data)
	return err
}

// MarshalYAML writes the workflow's fields in field order, then its extension
// keys as written.
func (w Workflow) MarshalYAML() (any, error) {
	type workflow Workflow
	return members(workflow(w), w.Extensions), nil
}

func (t *Task) UnmarshalYAML(data []byte) error {
	type task Task
	var fields task
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*t = Task(fields)
	var err error
	t.Extensions, err = extensions(data)
	return err
}

func (t Task) MarshalYAML() (any, error) {
	type task Task
	return members(task(t), t.Extensions), nil
}

func (s *MCPServer) UnmarshalYAML(data []byte) error {
	type server MCPServer
	var fields server
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return err
	}
	*s = MCPServer(fields)
	s.Rest = s.Rest.without(s)
	var err error
	s.Extensions, err = extensions(data)
	return err
}

func (s MCPServer) MarshalYAML() (any, error) {
	type server MCPServer
	return members(server(s), s.Extensions), nil
}

// extensions reads the members of the object in data whose names start with
// x-, keeping their order and the order of the mappings inside their values.
func extensions(data []byte) (Extensions, error) {
	var all yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(data, &all, yaml.UseOrderedMap()); err != nil {
		return nil, err
	}
	var out Extensions
	for _, item := range all {
		if isExtension(item.Key) {
			out = append(out, item)
		}
	}
	return out, nil
}

func isExtension(key any) bool {
	name, ok := key.(string)
	return ok && strings.HasPrefix(name, "x-")
}

// members lists the fields of the struct v in field order as a mapping,
// leaving out empty omitempty fields, then appends ext. An inline Fields map
// adds its members in sorted order.
func members(v any, ext Extensions) yaml.MapSlice {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	var out yaml.MapSlice
	for i := range rt.NumField() {
		tag := rt.Field(i).Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		field := rv.Field(i)
		if options == "inline" {
			rest, _ := field.Interface().(Fields)
			for _, key := range slices.Sorted(maps.Keys(rest)) {
				out = append(out, yaml.MapItem{Key: key, Value: rest[key]})
			}
			continue
		}
		if options == "omitempty" && empty(field) {
			continue
		}
		out = append(out, yaml.MapItem{Key: name, Value: field.Interface()})
	}
	return append(out, ext...)
}

func empty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Slice, reflect.Map, reflect.String:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	}
	return v.IsZero()
}

// without drops the members that the struct v points at holds in fields of
// its own, and its extension keys. An inline map receives every member, not
// only the unknown ones.
func (f Fields) without(v any) Fields {
	t := reflect.TypeOf(v).Elem()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		delete(f, name)
	}
	for key := range f {
		if isExtension(key) {
			delete(f, key)
		}
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
		var err error
		if e.Extensions, err = extensions(data); err != nil {
			return err
		}
	default:
		return fmt.Errorf("edge %T: %w", node, ErrNotAnEdge)
	}
	return nil
}

func (e Edge) MarshalYAML() (any, error) {
	if e.Retries == nil && e.Session == "" && len(e.Extensions) == 0 {
		return e.Task, nil
	}
	type edge Edge
	return members(edge(e), e.Extensions), nil
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
		if len(names) == 0 {
			return fmt.Errorf("model: %w", ErrNoModels)
		}
		for i, name := range names {
			if slices.Contains(names[:i], name) {
				return fmt.Errorf("model %q: %w", name, ErrDuplicateModel)
			}
		}
		*m = names
	default:
		return fmt.Errorf("model %T: %w", node, ErrNotModel)
	}
	return nil
}

func (m *Metadata) UnmarshalYAML(data []byte) error {
	var items yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(data, &items, yaml.UseOrderedMap()); err != nil {
		return err
	}
	*m = Metadata(items)
	return nil
}

func (m Metadata) MarshalYAML() (any, error) {
	return yaml.MapSlice(m), nil
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
