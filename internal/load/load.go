package load

import (
	"os"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/schema"
)

func Task(path string) (*manifest.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	front, body, err := manifest.SplitTask(data)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateTask(front, body); err != nil {
		return nil, err
	}
	return manifest.UnmarshalTask(data)
}

func Workflow(path string) (*awf.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := schema.Validate(data); err != nil {
		return nil, err
	}
	doc, err := manifest.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return compile(doc)
}
