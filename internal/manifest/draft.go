package manifest

import (
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/version"
)

// SpecVersion is the OpenAWF version a document written here declares.
const SpecVersion Version = version.Spec

const placeholder = `# %s

Describe the work this task performs, and what it returns.
`

// NewWorkflow is a blank workflow that validates: one task, which ends the
// flow.
func NewWorkflow(id string, version Version) *Workflow {
	const start = "start"

	return &Workflow{
		OpenAWF: SpecVersion,
		Name:    id,
		Summary: "What this workflow does.",
		Version: version,
		Start:   start,
		Orchestration: Orchestration{
			start: Transition{},
		},
		Tasks: Tasks{
			start: TaskEntry{Task: NewTask(start, version)},
		},
	}
}

// NewTask is a blank task.
func NewTask(id string, version Version) *Task {
	return &Task{
		Version: version,
		Summary: "What this task does.",
		Body:    fmt.Sprintf(placeholder, id),
	}
}

// Redraft reissues the workflow under a new version. The sha describes the
// content it was taken from, so a draft carries none.
func (w *Workflow) Redraft(version Version) {
	w.Version = version
	w.SHA = nil
}

// Redraft reissues the task under a new version.
func (t *Task) Redraft(version Version) {
	t.Version = version
	t.SHA = nil
}
