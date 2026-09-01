package awf

// StuckStates are the states the flow can enter but never leave: no ending
// state is reachable from them, so a run that arrives loops forever. A retry
// cycle with no way out is the usual cause.
//
// States the flow never enters are not reported. They cannot trap a run.
func (w *Workflow) StuckStates() []*State {
	if w.Start == nil {
		return nil
	}
	live := reach(ends(w.States), reverse(w.States))
	entered := reach([]*State{w.Start}, next)

	var out []*State
	for _, s := range w.States {
		if entered[s] && !live[s] {
			out = append(out, s)
		}
	}
	return out
}

// ends are the states the flow stops at.
func ends(states []*State) []*State {
	var out []*State
	for _, s := range states {
		if len(s.Out) == 0 {
			out = append(out, s)
		}
	}
	return out
}

func next(s *State) []*State {
	out := make([]*State, 0, len(s.Out))
	for _, e := range s.Out {
		out = append(out, e.To)
	}
	return out
}

// reverse turns the machine round, so a walk from the ends finds every state
// that can get to one.
func reverse(states []*State) func(*State) []*State {
	back := make(map[*State][]*State, len(states))
	for _, s := range states {
		for _, e := range s.Out {
			back[e.To] = append(back[e.To], s)
		}
	}
	return func(s *State) []*State { return back[s] }
}

func reach(from []*State, step func(*State) []*State) map[*State]bool {
	seen := make(map[*State]bool, len(from))
	queue := make([]*State, 0, len(from))
	for _, s := range from {
		seen[s] = true
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, to := range step(s) {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}
