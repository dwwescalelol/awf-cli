package awf

import "testing"

// machine wires states named by letter, each entry listing the states it goes
// to. A state with no targets ends the flow.
func machine(start string, edges map[string][]string) *Workflow {
	states := make(map[string]*State, len(edges))
	for name := range edges {
		states[name] = &State{Task: &Task{Name: name}}
	}
	for name, to := range edges {
		for _, t := range to {
			states[name].Out = append(states[name].Out, Edge{To: states[t]})
		}
	}
	w := &Workflow{Start: states[start]}
	for _, s := range states {
		w.States = append(w.States, s)
	}
	return w
}

func names(states []*State) map[string]bool {
	out := make(map[string]bool, len(states))
	for _, s := range states {
		out[s.Task.Name] = true
	}
	return out
}

func TestStuckStates(t *testing.T) {
	tests := []struct {
		name  string
		start string
		edges map[string][]string
		want  []string
	}{
		{
			name:  "runs to an end",
			start: "a",
			edges: map[string][]string{"a": {"b"}, "b": {"c"}, "c": nil},
		},
		{
			name:  "loop with a way out",
			start: "a",
			edges: map[string][]string{"a": {"b"}, "b": {"a", "c"}, "c": nil},
		},
		{
			name:  "loop with no way out",
			start: "a",
			edges: map[string][]string{"a": {"b"}, "b": {"a"}},
			want:  []string{"a", "b"},
		},
		{
			name:  "one arm loops forever",
			start: "a",
			edges: map[string][]string{"a": {"b", "c"}, "b": nil, "c": {"d"}, "d": {"c"}},
			want:  []string{"c", "d"},
		},
		{
			name:  "unreachable loop is not stuck",
			start: "a",
			edges: map[string][]string{"a": {"b"}, "b": nil, "x": {"y"}, "y": {"x"}},
		},
		{
			name:  "start is the end",
			start: "a",
			edges: map[string][]string{"a": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(machine(tt.start, tt.edges).StuckStates())
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for _, name := range tt.want {
				if !got[name] {
					t.Errorf("%q not reported, got %v", name, got)
				}
			}
		})
	}
}
