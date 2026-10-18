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
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func sealCmd() *cobra.Command {
	workflow := documentCmd(store.Workflow, "Write a workflow's sha after dereferencing and validating it", runSeal)
	workflow.Long = `Write a workflow's sha after dereferencing and validating it.

Seal refuses a workflow whose $ref tasks are unsealed. ` + sealedNote + " " + readOnlyNote
	task := documentCmd(store.Task, "Write a task's sha after validating it", runSeal)
	task.Long = "Write a task's sha after validating it.\n\n" + sealedNote + " " + readOnlyNote
	return kindCmd(&cobra.Command{
		Use:   "seal",
		Short: "Write a document's sha after validating it",
		Long:  "Write a document's sha after validating it.\n\n" + sealedNote + " " + readOnlyNote,
	}, workflow, task)
}

const sealedNote = `A sealed document has a version. Sealing a draft moves it to its version's file,
which must be newer than the latest. Sealing is intended to be permanent. A
change requires a new draft.`

var (
	errNotNewer  = errors.New("not newer than the latest version")
	errNoVersion = errors.New("no version, a sealed document needs one")
)

func runSeal(_ *cobra.Command, r request) error {
	path, sha, err := sealDocument(r)
	if err != nil {
		return err
	}
	fmt.Println(path + "\nsealed " + sha)
	return nil
}

func sealDocument(r request) (string, string, error) {
	if err := writableTarget(r); err != nil {
		return "", "", err
	}
	d, err := resolve(r)
	if err != nil {
		return "", "", err
	}
	if d.at == nil {
		if d.at, err = d.scope.EntryAt(d.path); err != nil {
			return d.path, "", err
		}
	}
	if d.at != nil {
		if err := d.at.ID.CheckLocal(); err != nil {
			return d.path, "", err
		}
	}
	var data []byte
	var sha string
	var v version.Version
	if d.kind == store.Task {
		data, sha, v, err = sealTask(d)
	} else {
		data, sha, v, err = sealWorkflow(d)
	}
	if err != nil {
		return d.path, "", err
	}
	switch {
	case d.at != nil && d.at.Draft:
		if err := checkNewer(d, v); err != nil {
			return d.path, "", err
		}
		err = d.scope.Seal(d.kind, d.at.ID, v, data)
		return d.scope.Path(d.kind, d.at.ID, v), sha, err
	case d.at != nil:
		err = d.scope.Replace(d.kind, d.at.ID, d.at.Version, data)
	default:
		err = os.WriteFile(d.path, data, 0o644)
	}
	return d.path, sha, err
}

func checkNewer(d document, v version.Version) error {
	latest, err := d.scope.Latest(d.kind, d.at.ID)
	if errors.Is(err, store.ErrNotInstalled) {
		return nil
	}
	if err != nil {
		return err
	}
	if v.Compare(latest) <= 0 {
		return fmt.Errorf("%s %s: %w %s", d.at.ID, v, errNotNewer, latest)
	}
	return nil
}

func sealTask(d document) ([]byte, string, version.Version, error) {
	f, err := load.ReadTask(d.path, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", version.Version{}, err
	}
	f.Doc.SHA = nil
	if f.Doc.Version == (version.Version{}) {
		return nil, "", version.Version{}, fmt.Errorf("%s: %w", d.path, errNoVersion)
	}
	sha, err := seal.Task(f.Doc)
	if err != nil {
		return nil, "", version.Version{}, err
	}
	f.Doc.SHA = &sha
	data, err := manifest.MarshalTask(f.Doc)
	return data, sha, f.Doc.Version, err
}

func sealWorkflow(d document) ([]byte, string, version.Version, error) {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", version.Version{}, err
	}
	if names := unsealed(f); len(names) > 0 {
		return nil, "", version.Version{}, fmt.Errorf("%s: %w %s, seal them first", d.path, ErrUnsealedTasks, strings.Join(names, ", "))
	}
	f.Doc.SHA = nil
	if f.Doc.Version == (version.Version{}) {
		return nil, "", version.Version{}, fmt.Errorf("%s: %w", d.path, errNoVersion)
	}
	sha, err := seal.Workflow(f.Doc)
	if err != nil {
		return nil, "", version.Version{}, err
	}
	f.Doc.SHA = &sha
	data, err := manifest.Marshal(f.Doc)
	return data, sha, f.Doc.Version, err
}

var ErrUnsealedTasks = errors.New("unsealed tasks")

// unsealed names the $ref tasks without a sha.
func unsealed(f *load.WorkflowFile) []string {
	var names []string
	for name := range f.Sources {
		if f.Doc.Tasks[name].Task.SHA == nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func sealable(path string, sealed bool, err error) error {
	if sealed {
		return fmt.Errorf("%s: %w, make a new draft to change it", path, seal.ErrSealed)
	}
	return err
}
