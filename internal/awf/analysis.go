package awf

import (
	"errors"
	"fmt"
)

var (
	ErrNoTerminal = errors.New("no terminal task")
	ErrNoPath     = errors.New("no path to a terminal task")
)

func (w *Workflow) TrapStates() []*State {
	if w.Start == nil {
		return nil
	}
	live := reach(terminals(w.States), reverse(w.States))
	entered := reach([]*State{w.Start}, next)

	var out []*State
	for _, s := range w.States {
		if entered[s] && !live[s] {
			out = append(out, s)
		}
	}
	return out
}

// Check reports why the machine can never finish: it has no terminal state, or
// it has states that trap a run. One call answers the whole machine.
func (w *Workflow) Check() error {
	if w.Start == nil {
		return nil
	}
	if len(terminals(w.States)) == 0 {
		return fmt.Errorf("orchestration: %w", ErrNoTerminal)
	}

	trapped := w.TrapStates()
	errs := make([]error, 0, len(trapped))
	for _, s := range trapped {
		errs = append(errs, fmt.Errorf("orchestration/%s: %w", s.Task.Name, ErrNoPath))
	}
	return errors.Join(errs...)
}

func terminals(states []*State) []*State {
	var out []*State
	for _, s := range states {
		if s.IsTerminal() {
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
