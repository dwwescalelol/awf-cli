package awf

import (
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

type Workflow struct {
	Name    string
	Summary string
	Model   []string
	Effort  string
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
	On      *Outcome
	To      *State
	Retries *int
	Session Session
}

type Task struct {
	Name     string
	Version  version.Version
	Summary  string
	Model    []string
	Effort   string
	MaxTurns int
	Timeout  int
	Input    map[string]any
	Output   map[string]any
	Outcomes []Outcome
	Uses     []Use
	System   string
	Body     string
}

type MCPServer struct {
	Name      string
	Transport Transport
	Command   string
	Package   *Package
	Args      []string
	Env       []Var
	URL       string
	Headers   []Var
	OAuth     *OAuth
	Allow     Tools
	Deny      []string
	Timeout   int
}

// Permits reports whether the server lets a task call the tool: allow lists
// it, or allows every tool, and deny does not list it.
func (s *MCPServer) Permits(tool string) bool {
	if slices.Contains(s.Deny, tool) {
		return false
	}
	return s.Allow.All || slices.Contains(s.Allow.Names, tool)
}

// Package is a published package that provides a stdio server.
type Package struct {
	RegistryType string
	Identifier   string
	Version      string
	FileSHA256   string
}

type OAuth struct {
	ClientID     string
	Scopes       []string
	CallbackPort int
	MetadataURL  string
}

// Var is one member of an object of strings, such as env or headers.
type Var struct {
	Name  string
	Value string
}

type Tools struct {
	All   bool
	Names []string
}

// Use is what one entry of a task's uses grants: every tool Server allows
// when Tool is empty, or the one tool Tool.
type Use struct {
	Server *MCPServer
	Tool   string
}

// String returns the entry as written: the server name, or the qualified
// tool name <server>/<tool>.
func (u Use) String() string {
	if u.Tool == "" {
		return u.Server.Name
	}
	return u.Server.Name + "/" + u.Tool
}

// Outcome is a named result a task may report, with the JSON Schema of the
// data it returns.
type Outcome struct {
	Name   string
	Schema map[string]any
}

type Session string

const (
	// Fresh starts the task in a new agent session.
	Fresh Session = "fresh"
	// Resume continues the target task's own previous session.
	Resume Session = "resume"
)

type Transport string

const (
	Stdio Transport = "stdio"
	HTTP  Transport = "http"
	SSE   Transport = "sse"
)
