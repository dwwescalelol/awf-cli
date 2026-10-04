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
	On      Outcome
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
	Tools     Tools
}

type Tools struct {
	All   bool
	Names []string
}

type Outcome string

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
