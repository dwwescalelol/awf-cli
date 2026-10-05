package load

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

var (
	ErrNotATask      = errors.New("not a defined task")
	ErrNoNode        = errors.New("no orchestration node")
	ErrNotDeclared   = errors.New("not declared in mcp")
	ErrMustBranch    = errors.New("must branch")
	ErrNoOutcomes    = errors.New("task emits no outcomes")
	ErrNotAnOutcome  = errors.New("not an outcome")
	ErrNoEdge        = errors.New("no edge")
	ErrUnresolvedRef = errors.New("unresolved $ref")
)

// compile lowers a document into the machine it describes, binding every name
// to what it names. It reports every unbound name at once, and yields a
// machine only when all of them bind.
func compile(doc *manifest.Workflow) (*awf.Workflow, error) {
	servers := servers(doc.MCP)
	states, errs := states(doc, servers)

	for _, name := range sorted(doc.Orchestration) {
		state, ok := states[name]
		if !ok {
			errs = append(errs, fmt.Errorf("orchestration/%s: %w", name, ErrNotATask))
			continue
		}
		unresolved := doc.Tasks[name].Task == nil
		out, e := edges(name, doc.Orchestration[name], states, unresolved)
		state.Out, errs = out, append(errs, e...)
	}

	start, ok := states[doc.Start]
	if !ok {
		errs = append(errs, fmt.Errorf("start %q: %w", doc.Start, ErrNotATask))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	w := &awf.Workflow{
		Name:    doc.Name,
		Summary: doc.Summary,
		Model:   doc.Model,
		Version: doc.Version,
		Start:   start,
		States:  ordered(states),
		Servers: ordered(servers),
	}
	return w, w.Check()
}

func states(doc *manifest.Workflow, servers map[string]*awf.MCPServer) (map[string]*awf.State, []error) {
	out := make(map[string]*awf.State, len(doc.Tasks))
	var errs []error

	for _, name := range sorted(doc.Tasks) {
		entry := doc.Tasks[name]
		if _, ok := doc.Orchestration[name]; !ok {
			errs = append(errs, fmt.Errorf("tasks/%s: %w", name, ErrNoNode))
		}
		if entry.Task == nil {
			// bundle reported the $ref. A placeholder lets edges to it still bind.
			out[name] = &awf.State{Task: &awf.Task{Name: name}}
			continue
		}
		t, e := task(name, entry.Task, doc.Model, servers)
		out[name], errs = &awf.State{Task: t}, append(errs, e...)
	}
	return out, errs
}

func task(name string, doc *manifest.Task, model string, servers map[string]*awf.MCPServer) (*awf.Task, []error) {
	if doc.Model != "" {
		model = doc.Model
	}

	var errs []error
	uses := make([]*awf.MCPServer, 0, len(doc.Uses))
	for _, use := range doc.Uses {
		server, ok := servers[use]
		if !ok {
			errs = append(errs, fmt.Errorf("tasks/%s/uses %q: %w", name, use, ErrNotDeclared))
			continue
		}
		uses = append(uses, server)
	}

	outcomes := make([]awf.Outcome, 0, len(doc.Outcomes))
	for _, outcome := range doc.Outcomes {
		outcomes = append(outcomes, awf.Outcome(outcome))
	}

	return &awf.Task{
		Name:     name,
		Version:  doc.Version,
		Summary:  doc.Summary,
		Model:    model,
		Input:    doc.Input,
		Output:   doc.Output,
		Outcomes: outcomes,
		Uses:     uses,
		Body:     string(doc.Body),
	}, errs
}

// edges binds a transition to the states it leaves for. A task that emits
// outcomes branches on all of them; one that emits none ends the flow or takes
// a single edge. An unresolved task's outcomes are unknown, so only its targets
// bind.
func edges(name string, node manifest.Transition, states map[string]*awf.State, unresolved bool) ([]awf.Edge, []error) {
	path := "orchestration/" + name
	outcomes := states[name].Task.Outcomes

	switch {
	case unresolved:
		return targets(path, node, states)

	case node.Edge == nil && node.Branch == nil:
		if len(outcomes) > 0 {
			return nil, []error{fmt.Errorf("%s: %w on %q", path, ErrMustBranch, outcomes)}
		}
		return nil, nil

	case node.Edge != nil:
		if len(outcomes) > 0 {
			return nil, []error{fmt.Errorf("%s: %w on %q", path, ErrMustBranch, outcomes)}
		}
		e, errs := edge(path, "", *node.Edge, states)
		return []awf.Edge{e}, errs
	}

	if len(outcomes) == 0 {
		return nil, []error{fmt.Errorf("%s: %w", path, ErrNoOutcomes)}
	}

	var errs []error
	out := make([]awf.Edge, 0, len(node.Branch))
	for _, on := range sorted(node.Branch) {
		if !slices.Contains(outcomes, awf.Outcome(on)) {
			errs = append(errs, fmt.Errorf("%s/%s: %w of %q", path, on, ErrNotAnOutcome, name))
			continue
		}
		e, err := edge(path+"/"+on, awf.Outcome(on), node.Branch[on], states)
		out, errs = append(out, e), append(errs, err...)
	}
	for _, on := range outcomes {
		if _, ok := node.Branch[string(on)]; !ok {
			errs = append(errs, fmt.Errorf("%s/%s: %w", path, on, ErrNoEdge))
		}
	}
	return out, errs
}

func targets(path string, node manifest.Transition, states map[string]*awf.State) ([]awf.Edge, []error) {
	if node.Edge != nil {
		e, errs := edge(path, "", *node.Edge, states)
		return []awf.Edge{e}, errs
	}
	var errs []error
	out := make([]awf.Edge, 0, len(node.Branch))
	for _, on := range sorted(node.Branch) {
		e, err := edge(path+"/"+on, awf.Outcome(on), node.Branch[on], states)
		out, errs = append(out, e), append(errs, err...)
	}
	return out, errs
}

func edge(path string, on awf.Outcome, doc manifest.Edge, states map[string]*awf.State) (awf.Edge, []error) {
	to, ok := states[doc.Task]
	if !ok {
		return awf.Edge{}, []error{fmt.Errorf("%s %q: %w", path, doc.Task, ErrNotATask)}
	}
	session := awf.Session(doc.Session)
	if session == "" {
		session = awf.Fresh
	}
	return awf.Edge{On: on, To: to, Retries: doc.Retries, Session: session}, nil
}

func servers(docs map[string]manifest.MCPServer) map[string]*awf.MCPServer {
	out := make(map[string]*awf.MCPServer, len(docs))
	for name, doc := range docs {
		tools := awf.Tools{All: true}
		if doc.Tools != nil {
			tools = awf.Tools{All: doc.Tools.All, Names: doc.Tools.Names}
		}
		out[name] = &awf.MCPServer{Name: name, Transport: awf.Transport(doc.Transport), Tools: tools}
	}
	return out
}

func sorted[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func ordered[T any](m map[string]*T) []*T {
	out := make([]*T, 0, len(m))
	for _, name := range sorted(m) {
		out = append(out, m[name])
	}
	return out
}
