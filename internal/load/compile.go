package load

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

var (
	ErrNotATask     = errors.New("not a defined task")
	ErrNoNode       = errors.New("no orchestration node")
	ErrNotDeclared  = errors.New("not declared in mcp")
	ErrNoTool       = errors.New("names no tool after the /")
	ErrNotAllowed   = errors.New("not in its server's allow list")
	ErrDenied       = errors.New("listed in its server's deny list")
	ErrMustBranch   = errors.New("must branch")
	ErrNoOutcomes   = errors.New("task emits no outcomes")
	ErrNotAnOutcome = errors.New("not an outcome")
	ErrNoEdge       = errors.New("no edge")
)

// compile lowers a document into the machine it describes, binding every name
// to what it names. It reports every unbound name at once, and yields a
// machine only when all of them bind.
func compile(doc *manifest.Workflow) (*awf.Workflow, []error) {
	servers, errs := servers(doc.MCP)
	states, e := states(doc, servers)
	errs = append(errs, e...)

	for _, name := range sorted(doc.Orchestration) {
		state, ok := states[name]
		if !ok {
			errs = append(errs, onKey("orchestration/"+name, fmt.Errorf("orchestration/%s: %w", name, ErrNotATask)))
			continue
		}
		unresolved := doc.Tasks[name].Task == nil
		out, e := edges(name, doc.Orchestration[name], states, unresolved)
		state.Out, errs = out, append(errs, e...)
	}

	start, ok := states[doc.Start]
	if !ok {
		errs = append(errs, onValue("start", fmt.Errorf("start %q: %w", doc.Start, ErrNotATask)))
	}

	if len(errs) > 0 {
		return nil, errs
	}

	w := &awf.Workflow{
		Name:    doc.Name,
		Summary: doc.Summary,
		Model:   doc.Model,
		Effort:  string(doc.Effort),
		Version: doc.Version,
		Start:   start,
		States:  ordered(states),
		Servers: ordered(servers),
	}
	for _, err := range w.Check() {
		if _, ok := err.(*awf.StateError); !ok {
			err = onKey("orchestration", err)
		}
		errs = append(errs, err)
	}
	return w, errs
}

func states(doc *manifest.Workflow, servers map[string]*awf.MCPServer) (map[string]*awf.State, []error) {
	out := make(map[string]*awf.State, len(doc.Tasks))
	var errs []error

	for _, name := range sorted(doc.Tasks) {
		entry := doc.Tasks[name]
		if _, ok := doc.Orchestration[name]; !ok {
			errs = append(errs, onKey("tasks/"+name, fmt.Errorf("tasks/%s: %w", name, ErrNoNode)))
		}
		if entry.Task == nil {
			// bundle reported the $ref. A placeholder lets edges to it still bind.
			out[name] = &awf.State{Task: &awf.Task{Name: name}}
			continue
		}
		t, e := task(name, entry.Task, doc, servers)
		out[name], errs = &awf.State{Task: t}, append(errs, e...)
	}
	return out, errs
}

// task lowers one task. It inherits the workflow's model and effort when it
// sets neither of its own.
func task(name string, doc *manifest.Task, wf *manifest.Workflow, servers map[string]*awf.MCPServer) (*awf.Task, []error) {
	model, effort := wf.Model, wf.Effort
	if len(doc.Model) > 0 {
		model = doc.Model
	}
	if doc.Effort != "" {
		effort = doc.Effort
	}

	var errs []error
	uses := make([]awf.Use, 0, len(doc.Uses))
	for i, entry := range doc.Uses {
		use, err := grant(entry, servers)
		if err != nil {
			errs = append(errs, onValue(fmt.Sprintf("tasks/%s/uses/%d", name, i), fmt.Errorf("tasks/%s/uses %q: %w", name, entry, err)))
			continue
		}
		uses = append(uses, use)
	}

	errs = append(errs, params("tasks/"+name+"/", doc, inYAML("tasks/"+name+"/"))...)

	outcomes := make([]awf.Outcome, 0, len(doc.Outcomes))
	for _, outcome := range doc.Outcomes {
		outcomes = append(outcomes, awf.Outcome{Name: outcome.Name, Schema: outcome.Schema})
	}

	return &awf.Task{
		Name:     name,
		Version:  doc.Version,
		Summary:  doc.Summary,
		Model:    model,
		Effort:   string(effort),
		MaxTurns: doc.MaxTurns,
		Timeout:  doc.Timeout,
		Input:    doc.Input,
		Output:   doc.Output,
		Outcomes: outcomes,
		Uses:     uses,
		System:   string(doc.System),
		Body:     string(doc.Body),
	}, errs
}

// edges binds a transition to the states it leaves for. A task that emits
// outcomes branches on all of them; one that emits none ends the flow or takes
// a single edge. An unresolved task's outcomes are unknown, so only its targets
// bind.
func edges(name string, node manifest.Transition, states map[string]*awf.State, unresolved bool) ([]awf.Edge, []error) {
	path := "orchestration/" + name
	outcomes := names(states[name].Task.Outcomes)

	switch {
	case unresolved:
		return targets(path, node, states)

	case node.Edge == nil && node.Branch == nil:
		if len(outcomes) > 0 {
			return nil, []error{onKey(path, fmt.Errorf("%s: %w on %q", path, ErrMustBranch, outcomes))}
		}
		return nil, nil

	case node.Edge != nil:
		if len(outcomes) > 0 {
			return nil, []error{onKey(path, fmt.Errorf("%s: %w on %q", path, ErrMustBranch, outcomes))}
		}
		e, errs := edge(path, nil, *node.Edge, states)
		return []awf.Edge{e}, errs
	}

	if len(outcomes) == 0 {
		return nil, []error{onKey(path, fmt.Errorf("%s: %w", path, ErrNoOutcomes))}
	}

	declared := states[name].Task.Outcomes
	var errs []error
	out := make([]awf.Edge, 0, len(node.Branch))
	for _, on := range sorted(node.Branch) {
		i := slices.IndexFunc(declared, func(o awf.Outcome) bool { return o.Name == on })
		if i < 0 {
			errs = append(errs, onKey(path+"/"+on, fmt.Errorf("%s/%s: %w of %q", path, on, ErrNotAnOutcome, name)))
			continue
		}
		e, err := edge(path+"/"+on, &declared[i], node.Branch[on], states)
		out, errs = append(out, e), append(errs, err...)
	}
	for _, o := range declared {
		if _, ok := node.Branch[o.Name]; !ok {
			errs = append(errs, onKey(path+"/"+o.Name, fmt.Errorf("%s/%s: %w", path, o.Name, ErrNoEdge)))
		}
	}
	return out, errs
}

func targets(path string, node manifest.Transition, states map[string]*awf.State) ([]awf.Edge, []error) {
	if node.Edge != nil {
		e, errs := edge(path, nil, *node.Edge, states)
		return []awf.Edge{e}, errs
	}
	var errs []error
	out := make([]awf.Edge, 0, len(node.Branch))
	for _, on := range sorted(node.Branch) {
		e, err := edge(path+"/"+on, &awf.Outcome{Name: on}, node.Branch[on], states)
		out, errs = append(out, e), append(errs, err...)
	}
	return out, errs
}

func edge(path string, on *awf.Outcome, doc manifest.Edge, states map[string]*awf.State) (awf.Edge, []error) {
	to, ok := states[doc.Task]
	if !ok {
		return awf.Edge{}, []error{onValue(path, fmt.Errorf("%s %q: %w", path, doc.Task, ErrNotATask))}
	}
	session := awf.Session(doc.Session)
	if session == "" {
		session = awf.Fresh
	}
	return awf.Edge{On: on, To: to, Retries: doc.Retries, Session: session}, nil
}

// grant binds one entry of a task's uses: a server name, or a qualified tool
// name whose server is the text before the first /.
func grant(entry string, servers map[string]*awf.MCPServer) (awf.Use, error) {
	name, tool, qualified := strings.Cut(entry, "/")
	server, ok := servers[name]
	switch {
	case !ok && qualified:
		return awf.Use{}, fmt.Errorf("server %q: %w", name, ErrNotDeclared)
	case !ok:
		return awf.Use{}, ErrNotDeclared
	case qualified && tool == "":
		return awf.Use{}, ErrNoTool
	case qualified && slices.Contains(server.Deny, tool):
		return awf.Use{}, fmt.Errorf("tool %q: %w", tool, ErrDenied)
	case qualified && !server.Permits(tool):
		return awf.Use{}, fmt.Errorf("tool %q: %w", tool, ErrNotAllowed)
	}
	return awf.Use{Server: server, Tool: tool}, nil
}

// servers lowers the declared servers. It reports each malformed placeholder
// in their args, env and headers.
func servers(docs map[string]manifest.MCPServer) (map[string]*awf.MCPServer, []error) {
	out := make(map[string]*awf.MCPServer, len(docs))
	var errs []error
	for _, name := range sorted(docs) {
		doc := docs[name]
		for _, value := range doc.Credentials() {
			if _, err := manifest.Placeholders(value.Value); err != nil {
				path := "mcp/" + name + "/" + value.Key
				errs = append(errs, onValue(path, fmt.Errorf("%s: %w", path, err)))
			}
		}
		out[name] = server(name, doc)
	}
	return out, errs
}

func server(name string, doc manifest.MCPServer) *awf.MCPServer {
	allow := awf.Tools{All: true}
	if doc.Allow != nil {
		allow = awf.Tools{All: doc.Allow.All, Names: doc.Allow.Names}
	}
	s := &awf.MCPServer{
		Name:      name,
		Transport: awf.Transport(doc.Type),
		Command:   doc.Command,
		Args:      doc.Args,
		Env:       vars(doc.Env),
		URL:       doc.URL,
		Headers:   vars(doc.Headers),
		Allow:     allow,
		Deny:      doc.Deny,
		Timeout:   doc.Timeout,
	}
	if p := doc.Package; p != nil {
		s.Package = &awf.Package{RegistryType: p.RegistryType, Identifier: p.Identifier, Version: p.Version, FileSHA256: p.FileSHA256}
	}
	if o := doc.OAuth; o != nil {
		s.OAuth = &awf.OAuth{ClientID: o.ClientID, Scopes: o.Scopes, CallbackPort: o.CallbackPort, MetadataURL: o.MetadataURL}
	}
	return s
}

func vars(m manifest.StringMap) []awf.Var {
	if len(m) == 0 {
		return nil
	}
	out := make([]awf.Var, len(m))
	for i, kv := range m {
		out[i] = awf.Var{Name: kv.Key, Value: kv.Value}
	}
	return out
}

func names(outcomes []awf.Outcome) []string {
	out := make([]string, len(outcomes))
	for i, o := range outcomes {
		out[i] = o.Name
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
