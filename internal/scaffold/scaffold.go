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

func Create(s *store.Store, kind store.DocumentKind, id store.ID) error {
	if err := id.CheckLocal(); err != nil {
		return err
	}
	previous, err := s.Latest(kind, id)
	fresh := errors.Is(err, store.ErrNotInstalled)
	if err != nil && !fresh {
		return err
	}

	var data []byte
	switch {
	case fresh:
		data, err = Blank(kind, id, version.Version{})
	case kind == store.Task:
		data, err = draft(s, kind, id, previous, manifest.UnmarshalTask, manifest.MarshalTask, func(t *manifest.Task) {
			t.Version, t.SHA = version.Version{}, nil
		})
	default:
		data, err = draft(s, kind, id, previous, manifest.Unmarshal, manifest.Marshal, func(wf *manifest.Workflow) {
			wf.Version, wf.SHA = version.Version{}, nil
		})
	}
	if err != nil {
		return err
	}
	return s.WriteDraft(kind, id, data)
}

func CopyTask(s *store.Store, id store.ID, doc *manifest.Task) error {
	doc.Version, doc.SHA = version.Version{}, nil
	data, err := manifest.MarshalTask(doc)
	if err != nil {
		return err
	}
	return s.WriteDraft(store.Task, id, data)
}

func CopyWorkflow(s *store.Store, id store.ID, doc *manifest.Workflow) error {
	doc.Name, doc.Version, doc.SHA = id.String(), version.Version{}, nil
	data, err := manifest.Marshal(doc)
	if err != nil {
		return err
	}
	return s.WriteDraft(store.Workflow, id, data)
}

func Blank(kind store.DocumentKind, id store.ID, v version.Version) ([]byte, error) {
	if kind == store.Task {
		t := blankTask(id.String())
		t.Version = v
		if err := sealBlank(v, &t.SHA, func() (string, error) { return seal.Task(t) }); err != nil {
			return nil, err
		}
		return manifest.MarshalTask(t)
	}
	wf := blankWorkflow(id.String())
	wf.Version = v
	if err := sealBlank(v, &wf.SHA, func() (string, error) { return seal.Workflow(wf) }); err != nil {
		return nil, err
	}
	return manifest.Marshal(wf)
}

func sealBlank(v version.Version, sha **string, hash func() (string, error)) error {
	if v == (version.Version{}) {
		return nil
	}
	h, err := hash()
	if err != nil {
		return err
	}
	*sha = &h
	return nil
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

func blankWorkflow(id string) *manifest.Workflow {
	const start = "start"
	task := blankTask(start)
	task.OpenAWF = version.Version{}
	return &manifest.Workflow{
		OpenAWF:       schema.SpecVersion,
		Name:          id,
		Start:         start,
		Orchestration: manifest.Orchestration{start: manifest.Transition{}},
		Tasks:         manifest.Tasks{start: manifest.TaskEntry{Task: task}},
	}
}

func blankTask(id string) *manifest.Task {
	return &manifest.Task{
		OpenAWF: schema.SpecVersion,
		Summary: "What this task does.",
		Body:    manifest.Body(fmt.Sprintf("# %s\n\nDescribe the work this task performs, and what it returns.\n", id)),
	}
}
