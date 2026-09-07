package manifest

import (
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

const placeholder = `# %s

Describe the work this task performs, and what it returns.
`

// NewWorkflow is a blank workflow that validates: one task, which ends the
// flow.
func NewWorkflow(id string, v version.Version) *Workflow {
	const start = "start"

	return &Workflow{
		OpenAWF: version.SpecVersion,
		Name:    id,
		Summary: "What this workflow does.",
		Version: v,
		Start:   start,
		Orchestration: Orchestration{
			start: Transition{},
		},
		Tasks: Tasks{
			start: TaskEntry{Task: NewTask(start, v)},
		},
	}
}

// NewTask is a blank task.
func NewTask(id string, v version.Version) *Task {
	return &Task{
		Version: v,
		Summary: "What this task does.",
		Body:    fmt.Sprintf(placeholder, id),
	}
}

// Redraft reissues the workflow under a new version. The sha describes the
// content it was taken from, so a draft carries none.
func (w *Workflow) Redraft(v version.Version) {
	w.Version = v
	w.SHA = nil
}

// Redraft reissues the task under a new version.
func (t *Task) Redraft(v version.Version) {
	t.Version = v
	t.SHA = nil
}
