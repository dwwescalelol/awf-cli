package awf

import "github.com/dwwescalelol/awf-cli/internal/version"

type Workflow struct {
	Name    string
	Summary string
	Model   string
	Version version.Version
	Start   *State
	States  []*State
	Servers []*MCPServer
}

type State struct {
	Task *Task
	Out  []Edge
}

func (s *State) IsTerminal() bool {
	return len(s.Out) == 0
}

type Edge struct {
	On      string
	To      *State
	Retries *int
	Session Session
}

type Task struct {
	Name     string
	Version  version.Version
	Summary  string
	Model    string
	Input    map[string]any
	Output   map[string]any
	Outcomes []Outcome
	Uses     []*MCPServer
	Body     string
}

type MCPServer struct {
	Name      string
	Transport Transport
	Allow     Tools
	Deny      []string
}

type Tools struct {
	All   bool
	Names []string
}

// Outcome is a named result a task may report, with the JSON Schema of the
// data it returns.
type Outcome struct {
	Name   string
	Schema map[string]any
}

type Session string

const (
	Fresh  Session = "fresh"
	Resume Session = "resume"
)

type Transport string

const (
	Stdio Transport = "stdio"
	HTTP  Transport = "http"
	SSE   Transport = "sse"
)
