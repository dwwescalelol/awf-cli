package manifest

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/awf"
)

// Validate reports everything wrong with the document, so one call answers the
// whole file. A document that validates can be transformed without checks.
func (w *Workflow) Validate() error {
	var v checks

	for _, name := range sorted(w.Tasks) {
		entry := w.Tasks[name]
		if entry.Task == nil {
			continue // a $ref, not handled yet
		}
		v.task(name, entry.Task, w.MCP)
		if _, ok := w.Orchestration[name]; !ok {
			v.failf("tasks/"+name, "task has no orchestration node, so it never runs")
		}
	}

	if _, ok := w.Tasks[w.Start]; !ok {
		v.failf("start", "%q is not a defined task", w.Start)
	}

	for _, name := range sorted(w.Orchestration) {
		v.node(name, w.Orchestration[name], w.Tasks)
	}

	return errors.Join(v...)
}

// Graph reports what is wrong with the machine the document describes: no task
// ends the flow, or tasks the flow can enter but never leave. It assumes the
// document validates, so call Validate first.
func (w *Workflow) Graph() error {
	var v checks
	for _, err := range unjoin(w.Build().Check()) {
		var trap *awf.TrapError
		switch {
		case errors.Is(err, awf.ErrNoTerminal):
			v.failf("orchestration", "no task ends the flow, so it can never finish")
		case errors.As(err, &trap):
			v.failf("orchestration/"+trap.State.Task.Name, "the flow can never leave this task")
		default:
			v = append(v, err)
		}
	}
	return errors.Join(v...)
}

// unjoin splits an error joined by errors.Join back into its parts.
func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	if err == nil {
		return nil
	}
	return []error{err}
}

// node checks that a transition leaves for somewhere real, and that it agrees
// with the outcomes its task emits.
func (v *checks) node(name string, node Transition, tasks Tasks) {
	path := "orchestration/" + name

	entry, ok := tasks[name]
	if !ok {
		v.failf(path, "no task %q is defined", name)
		return
	}
	if entry.Task == nil {
		return // a $ref, not handled yet
	}
	outcomes := entry.Task.Outcomes

	switch {
	case node.Terminal():
		if len(outcomes) > 0 {
			v.failf(path, "task emits outcomes, so the flow cannot end here")
		}
		return

	case node.Edge != nil:
		if len(outcomes) > 0 {
			v.failf(path, "task emits outcomes, so the node must branch")
			return
		}
		v.edge(path, *node.Edge, tasks)
		return
	}

	if len(outcomes) == 0 {
		v.failf(path, "node branches, but the task emits no outcomes")
		return
	}
	for _, outcome := range sorted(node.Branch) {
		if !slices.Contains(outcomes, outcome) {
			v.failf(path+"/"+outcome, "%q is not an outcome of the task", outcome)
			continue
		}
		v.edge(path+"/"+outcome, node.Branch[outcome], tasks)
	}
	for _, outcome := range outcomes {
		if _, ok := node.Branch[outcome]; !ok {
			v.failf(path, "task emits %q, which has no edge", outcome)
		}
	}
}

func (v *checks) edge(path string, e Edge, tasks Tasks) {
	if _, ok := tasks[e.Task]; !ok {
		v.failf(path, "edge to undefined task %q", e.Task)
	}
	switch e.Session {
	case "", "fresh", "resume":
	default:
		v.failf(path+"/session", "%q is not fresh or resume", e.Session)
	}
	if e.Retries != nil && *e.Retries < 0 {
		v.failf(path+"/retries", "%d is negative", *e.Retries)
	}
}

func (v *checks) task(name string, t *Task, servers map[string]MCPServer) {
	path := "tasks/" + name
	if t.Body == "" {
		v.failf(path+"/body", "task has no body")
	}
	for _, server := range t.Uses {
		if _, ok := servers[server]; !ok {
			v.failf(path+"/uses", "%q is not declared in mcp", server)
		}
	}
}

type checks []error

func (v *checks) failf(path, format string, args ...any) {
	*v = append(*v, &ValidationError{Path: "/" + path, Msg: fmt.Sprintf(format, args...)})
}

func sorted[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
