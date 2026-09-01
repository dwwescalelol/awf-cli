package validate

import (
	"maps"
	"slices"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

// graph checks the rules the schema cannot express and returns the task names
// held as a $ref.
func graph(d *diags, wf *manifest.Workflow) []string {
	var unresolved []string
	for _, name := range sortedKeys(wf.Tasks) {
		if wf.Tasks[name].Task == nil {
			unresolved = append(unresolved, name)
		}
		if _, ok := wf.Orchestration[name]; !ok {
			d.warnf("/tasks/"+name, "task has no orchestration node")
		}
	}
	if _, ok := wf.Tasks[wf.Start]; !ok {
		d.errorf("/start", "%q is not a defined task", wf.Start)
	}

	terminal := false
	for _, name := range sortedKeys(wf.Orchestration) {
		node := wf.Orchestration[name]
		path := "/orchestration/" + name
		terminal = terminal || node.Terminal()

		entry, defined := wf.Tasks[name]
		if !defined {
			d.errorf(path, "no task %q is defined", name)
		}
		for edgePath, edge := range edges(path, node) {
			if _, ok := wf.Tasks[edge.Task]; !ok {
				d.errorf(edgePath, "edge to undefined task %q", edge.Task)
			}
		}
		if entry.Task != nil {
			checkOutcomes(d, path, name, entry.Task.Outcomes, node)
		}
	}
	if !terminal {
		d.errorf("/orchestration", "no node is terminal, so the flow never ends")
	}

	checkMCP(d, wf, len(unresolved) == 0)
	checkFlow(d, wf)
	return unresolved
}

func checkOutcomes(d *diags, path, name string, outcomes []string, node manifest.Transition) {
	switch {
	case len(outcomes) > 0 && node.Edge != nil:
		d.errorf(path, "task %q declares outcomes, so its node must branch", name)
		return
	case len(outcomes) == 0 && node.Branch != nil:
		d.errorf(path, "node branches, but task %q declares no outcomes", name)
		return
	}
	for _, outcome := range sortedKeys(node.Branch) {
		if !slices.Contains(outcomes, outcome) {
			d.errorf(path+"/"+outcome, "%q is not an outcome of task %q", outcome, name)
		}
	}
	if node.Branch != nil {
		for _, outcome := range outcomes {
			if _, ok := node.Branch[outcome]; !ok {
				d.errorf(path, "task %q emits %q, which has no edge", name, outcome)
			}
		}
	}
}

func checkMCP(d *diags, wf *manifest.Workflow, resolved bool) {
	used := make(map[string]bool, len(wf.MCP))
	for _, name := range sortedKeys(wf.Tasks) {
		task := wf.Tasks[name].Task
		if task == nil {
			continue
		}
		for _, server := range task.Uses {
			if _, ok := wf.MCP[server]; !ok {
				d.errorf("/tasks/"+name+"/uses", "%q is not declared in mcp", server)
			}
			used[server] = true
		}
	}
	// A $ref task's uses are unknown, so no server can be called unused.
	if !resolved {
		return
	}
	for _, server := range sortedKeys(wf.MCP) {
		if !used[server] {
			d.warnf("/mcp/"+server, "no task uses this server")
		}
	}
}

// checkFlow reports what the graph's shape hides: tasks the flow never enters,
// and tasks it can never leave. Liveness is only meaningful for tasks the flow
// reaches, so an unreachable dead end is reported once, as unreachable.
func checkFlow(d *diags, wf *manifest.Workflow) {
	if _, ok := wf.Orchestration[wf.Start]; !ok {
		return
	}
	reachable := walk(map[string]bool{wf.Start: true}, successors(wf))

	terminals := make(map[string]bool)
	for name, node := range wf.Orchestration {
		if node.Terminal() {
			terminals[name] = true
		}
	}
	live := walk(terminals, predecessors(wf))

	for _, name := range sortedKeys(wf.Orchestration) {
		switch {
		case !reachable[name]:
			d.warnf("/orchestration/"+name, "task is not reachable from start")
		case !live[name]:
			d.errorf("/orchestration/"+name, "no terminal is reachable from task %q", name)
		}
	}
}

func walk(seed map[string]bool, next map[string][]string) map[string]bool {
	seen := maps.Clone(seed)
	queue := sortedKeys(seed)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, to := range next[name] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

func successors(wf *manifest.Workflow) map[string][]string {
	out := make(map[string][]string, len(wf.Orchestration))
	for name, node := range wf.Orchestration {
		for _, edge := range edges("", node) {
			if _, ok := wf.Orchestration[edge.Task]; ok {
				out[name] = append(out[name], edge.Task)
			}
		}
	}
	return out
}

func predecessors(wf *manifest.Workflow) map[string][]string {
	out := make(map[string][]string, len(wf.Orchestration))
	for name, to := range successors(wf) {
		for _, t := range to {
			out[t] = append(out[t], name)
		}
	}
	return out
}

// edges maps the path of every edge out of a node to the edge itself.
func edges(path string, node manifest.Transition) map[string]manifest.Edge {
	if node.Edge != nil {
		return map[string]manifest.Edge{path: *node.Edge}
	}
	out := make(map[string]manifest.Edge, len(node.Branch))
	for outcome, edge := range node.Branch {
		out[path+"/"+outcome] = edge
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
