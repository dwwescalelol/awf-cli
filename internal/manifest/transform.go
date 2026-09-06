package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
)

func (w *Workflow) Build() *awf.Workflow {
	servers := make(map[string]*awf.MCPServer, len(w.MCP))
	for name, s := range w.MCP {
		servers[name] = server(name, s)
	}

	states := make(map[string]*awf.State, len(w.Orchestration))
	for name := range w.Orchestration {
		states[name] = &awf.State{Task: task(name, w.Tasks[name].Task, servers)}
	}
	for name, node := range w.Orchestration {
		states[name].Out = transitions(node, states)
	}

	version, _ := parseVersion(w.Version)
	return &awf.Workflow{
		Name:    w.Name,
		Summary: w.Summary,
		Model:   w.Model,
		Version: version,
		Start:   states[w.Start],
		States:  values(states),
		Servers: values(servers),
	}
}

func transitions(node Transition, states map[string]*awf.State) []awf.Edge {
	switch {
	case node.Terminal():
		return nil
	case node.Edge != nil:
		return []awf.Edge{edge("", *node.Edge, states)}
	}
	out := make([]awf.Edge, 0, len(node.Branch))
	for _, outcome := range sorted(node.Branch) {
		out = append(out, edge(awf.Outcome(outcome), node.Branch[outcome], states))
	}
	return out
}

func edge(on awf.Outcome, e Edge, states map[string]*awf.State) awf.Edge {
	session := awf.Session(e.Session)
	if session == "" {
		session = awf.Fresh
	}
	return awf.Edge{On: on, To: states[e.Task], Retries: e.Retries, Session: session}
}

func task(name string, t *Task, servers map[string]*awf.MCPServer) *awf.Task {
	version, _ := parseVersion(t.Version)
	uses := make([]*awf.MCPServer, 0, len(t.Uses))
	for _, server := range t.Uses {
		uses = append(uses, servers[server])
	}
	outcomes := make([]awf.Outcome, 0, len(t.Outcomes))
	for _, outcome := range t.Outcomes {
		outcomes = append(outcomes, awf.Outcome(outcome))
	}
	return &awf.Task{
		Name:     name,
		Version:  version,
		Summary:  t.Summary,
		Model:    t.Model,
		Input:    t.Input,
		Output:   t.Output,
		Outcomes: outcomes,
		Uses:     uses,
		Body:     t.Body,
	}
}

func server(name string, s MCPServer) *awf.MCPServer {
	tools := awf.Tools{All: true}
	if s.Tools != nil {
		tools = awf.Tools{All: s.Tools.All, Names: s.Tools.Names}
	}
	return &awf.MCPServer{Name: name, Transport: awf.Transport(s.Transport), Tools: tools}
}

func parseVersion(v Version) (awf.Version, error) {
	if v == "" {
		return awf.Version{}, nil
	}
	parts := strings.Split(string(v), ".")
	if len(parts) != 3 {
		return awf.Version{}, fmt.Errorf("%q: not major.minor.patch", v)
	}
	var out awf.Version
	into := []*int{&out.Major, &out.Minor, &out.Patch}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return awf.Version{}, fmt.Errorf("%q: not major.minor.patch", v)
		}
		*into[i] = n
	}
	return out, nil
}

func values[V any](m map[string]V) []V {
	out := make([]V, 0, len(m))
	for _, k := range sorted(m) {
		out = append(out, m[k])
	}
	return out
}
