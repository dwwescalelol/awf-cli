package awf

import (
	"errors"
	"fmt"
)

// ErrNoTerminal reports a machine with no terminal state: every state leaves by
// an edge, so no run of it can finish.
var ErrNoTerminal = errors.New("no task ends the flow")

// TrapError reports a state that traps a run. Terminal states exist, but none
// is reachable from this one, so a run that arrives loops forever. A retry
// cycle with no way out is the usual cause.
type TrapError struct {
	State *State
}

func (e *TrapError) Error() string {
	return fmt.Sprintf("the flow can never leave task %q", e.State.Task.Name)
}
