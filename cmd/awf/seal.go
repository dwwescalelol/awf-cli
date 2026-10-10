package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func sealCmd() *cobra.Command {
	workflow := documentCmd(store.Workflow, "Write a workflow's sha after dereferencing and validating it", runSeal)
	workflow.Long = `Write a workflow's sha after dereferencing and validating it.

Seal refuses a workflow whose $ref tasks are unsealed. --recursive seals them
first. ` + sealedNote
	workflow.Flags().BoolP("recursive", "r", false, "seal the workflow's unsealed $ref tasks first")
	task := documentCmd(store.Task, "Write a task's sha after validating it", runSeal)
	task.Long = "Write a task's sha after validating it.\n\n" + sealedNote
	return kindCmd(&cobra.Command{
		Use:   "seal",
		Short: "Write a document's sha after validating it",
		Long:  "Write a document's sha after validating it.\n\n" + sealedNote,
	}, workflow, task)
}

const sealedNote = "Sealing a document is intended to be permanent. A change requires a new version."

func runSeal(cmd *cobra.Command, r request) error {
	recursive, _ := cmd.Flags().GetBool("recursive")
	path, sha, err := sealDocument(r, recursive)
	if err != nil {
		return err
	}
	fmt.Println(path + "\nsealed " + sha)
	return nil
}

func sealDocument(r request, recursive bool) (string, string, error) {
	d, err := resolve(r)
	if err != nil {
		return "", "", err
	}
	var data []byte
	var sha string
	if d.kind == store.Task {
		data, sha, err = sealTask(d)
	} else {
		data, sha, err = sealWorkflow(d, recursive)
	}
	if err != nil {
		return d.path, "", err
	}
	if d.at != nil {
		err = d.scope.Replace(d.kind, d.at.ID, d.at.Version, data)
	} else {
		err = os.WriteFile(d.path, data, 0o644)
	}
	return d.path, sha, err
}

func sealTask(d document) ([]byte, string, error) {
	f, err := load.ReadTask(d.path, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", err
	}
	f.Doc.SHA = nil
	sha, err := seal.Task(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.MarshalTask(f.Doc)
	return data, sha, err
}

func sealWorkflow(d document, recursive bool) ([]byte, string, error) {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", err
	}
	if names, paths := unsealed(f); len(names) > 0 {
		if !recursive {
			return nil, "", fmt.Errorf("%s: %w %s, seal them first or use seal workflow --recursive", d.path, ErrUnsealedTasks, strings.Join(names, ", "))
		}
		for _, path := range paths {
			data, _, err := sealTask(document{kind: store.Task, path: path})
			if err != nil {
				return nil, "", err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return nil, "", err
			}
		}
		if f, err = load.ReadWorkflow(d.path, d.scope, d.at); err != nil {
			return nil, "", err
		}
	}
	f.Doc.SHA = nil
	sha, err := seal.Workflow(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.Marshal(f.Doc)
	return data, sha, err
}

var ErrUnsealedTasks = errors.New("unsealed tasks")

// unsealed names the $ref tasks without a sha, and the files they come from.
func unsealed(f *load.WorkflowFile) ([]string, []string) {
	var names, paths []string
	for name, path := range f.Sources {
		if f.Doc.Tasks[name].Task.SHA != nil {
			continue
		}
		names = append(names, name)
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	slices.Sort(names)
	slices.Sort(paths)
	return names, paths
}

func sealable(path string, sealed bool, err error) error {
	if sealed {
		return fmt.Errorf("%s: %w, make a new version to change it", path, seal.ErrSealed)
	}
	return err
}
