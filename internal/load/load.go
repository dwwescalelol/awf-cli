package load

import (
	"os"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
)

type WorkflowFile struct {
	Source   []byte
	Doc      *manifest.Workflow
	Compiled *awf.Workflow
}

type TaskFile struct {
	Source []byte
	Doc    *manifest.Task
}

func Task(path string) (*manifest.Task, error) {
	f, err := ReadTask(path)
	if err != nil {
		return nil, err
	}
	return f.Doc, nil
}

func Workflow(path string) (*awf.Workflow, error) {
	f, err := ReadWorkflow(path)
	if err != nil {
		return nil, err
	}
	return f.Compiled, nil
}

func ReadTask(path string) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &TaskFile{Source: data}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return f, err
	}
	invalid := schema.ValidateTask(front, body)
	if f.Doc, err = manifest.UnmarshalTask(data); err != nil {
		f.Doc = nil
		if invalid == nil {
			invalid = err
		}
	}
	return f, invalid
}

func ReadWorkflow(path string) (*WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &WorkflowFile{Source: data}
	_, invalid := schema.Validate(data)
	if f.Doc, err = manifest.Unmarshal(data); err != nil {
		f.Doc = nil
		if invalid == nil {
			invalid = err
		}
	}
	if invalid != nil {
		return f, invalid
	}
	if f.Compiled, err = compile(f.Doc); err != nil {
		f.Compiled = nil
		return f, err
	}
	return f, nil
}
