package scaffold

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

const First = "0.1.0"

var FirstVersion = version.MustParse(First)

func Create(s *store.Store, kind store.DocumentKind, id store.ID, want version.Version, pinned bool) (version.Version, error) {
	previous, err := s.Latest(kind, id)
	fresh := errors.Is(err, store.ErrNotInstalled)
	if err != nil && !fresh {
		return version.Version{}, err
	}

	v := nextVersion(fresh, previous)
	if pinned {
		v = want
	}

	var data []byte
	switch {
	case fresh && kind == store.Task:
		data, err = manifest.MarshalTask(blankTask(id.String(), v))
	case fresh:
		data, err = manifest.Marshal(blankWorkflow(id.String(), v))
	case kind == store.Task:
		data, err = draft(s, kind, id, previous, manifest.UnmarshalTask, manifest.MarshalTask, func(t *manifest.Task) {
			t.Version, t.SHA = v, nil
		})
	default:
		data, err = draft(s, kind, id, previous, manifest.Unmarshal, manifest.Marshal, func(wf *manifest.Workflow) {
			wf.Version, wf.SHA = v, nil
		})
	}
	if err != nil {
		return version.Version{}, err
	}
	if err := s.Write(kind, id, v, data); err != nil {
		return version.Version{}, err
	}
	return v, nil
}

// CopyTask writes doc as the task id at version v. A task's sha survives, since
// the version is not hashed.
func CopyTask(s *store.Store, id store.ID, v version.Version, doc *manifest.Task) error {
	doc.Version = v
	data, err := manifest.MarshalTask(doc)
	if err != nil {
		return err
	}
	return s.Write(store.Task, id, v, data)
}

// CopyWorkflow writes doc as the workflow id at version v. The name is hashed,
// so a renamed workflow loses its sha.
func CopyWorkflow(s *store.Store, id store.ID, v version.Version, doc *manifest.Workflow) error {
	doc.Name, doc.Version = id.String(), v
	if seal.CheckWorkflow(doc) != nil {
		doc.SHA = nil
	}
	data, err := manifest.Marshal(doc)
	if err != nil {
		return err
	}
	return s.Write(store.Workflow, id, v, data)
}

func nextVersion(fresh bool, previous version.Version) version.Version {
	if fresh {
		return FirstVersion
	}
	return previous.NextMinor()
}

func draft[T any](s *store.Store, kind store.DocumentKind, id store.ID, previous version.Version, unmarshal func([]byte) (*T, error), marshal func(*T) ([]byte, error), restamp func(*T)) ([]byte, error) {
	data, err := s.Read(kind, id, previous)
	if err != nil {
		return nil, err
	}
	doc, err := unmarshal(data)
	if err != nil {
		return nil, err
	}
	restamp(doc)
	return marshal(doc)
}

func blankWorkflow(id string, v version.Version) *manifest.Workflow {
	const start = "start"
	task := blankTask(start, v)
	task.OpenAWF, task.Version = version.Version{}, version.Version{}
	return &manifest.Workflow{
		OpenAWF:       schema.SpecVersion,
		Name:          id,
		Version:       v,
		Start:         start,
		Orchestration: manifest.Orchestration{start: manifest.Transition{}},
		Tasks:         manifest.Tasks{start: manifest.TaskEntry{Task: task}},
	}
}

func blankTask(id string, v version.Version) *manifest.Task {
	return &manifest.Task{
		OpenAWF: schema.SpecVersion,
		Version: v,
		Summary: "What this task does.",
		Body:    manifest.Body(fmt.Sprintf("# %s\n\nDescribe the work this task performs, and what it returns.\n", id)),
	}
}
