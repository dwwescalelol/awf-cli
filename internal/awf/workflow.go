// Package awf is the OpenAWF domain: a workflow as a finite state machine of
// tasks, resolved. It knows nothing about documents.
package awf

type Workflow struct {
	Name    string
	Summary string
	Model   string
	Version Version
	Start   *State
	States  []*State
	Servers []*MCPServer
}

// State runs a task, then leaves by the edge the task's outcome fires. A state
// with no edges out ends the flow.
type State struct {
	Task *Task
	Out  []Edge
}

type Edge struct {
	On      Outcome // the outcome that fires it, empty when the task emits none
	To      *State
	Retries *int
	Session Session
}

type Task struct {
	Name     string
	Version  Version
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

// Tools is the set of a server's tools the workflow may call: all of them, or
// a named few.
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

type Version struct {
	Major, Minor, Patch int
}
