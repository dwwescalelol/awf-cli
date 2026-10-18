package main

import (
	"errors"
	"fmt"
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
	workflow := sealKindCmd(store.Workflow, "Write a workflow's sha after dereferencing and validating it", `Write a workflow's sha after dereferencing and validating it.

Seal refuses a workflow whose $ref tasks are unsealed.

`+sealedNote+" "+readOnlyNote)
	task := sealKindCmd(store.Task, "Write a task's sha after validating it", "Write a task's sha after validating it.\n\n"+sealedNote+" "+readOnlyNote)
	return kindCmd(&cobra.Command{
		Use:   "seal",
		Short: "Write a draft's sha after validating it",
		Long:  "Write a draft's sha after validating it.\n\n" + sealedNote + " " + readOnlyNote,
	}, workflow, task)
}

func sealKindCmd(kind store.DocumentKind, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   kind.String() + " <id>",
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			path, sha, err := sealDocument(request{kind: kind, args: args, global: global})
			if err != nil {
				return err
			}
			fmt.Println(path + "\nsealed " + sha)
			return nil
		},
	}
}

const sealedNote = `Seal acts on the draft of <id>, and moves it to the file of the version
written in it, which must be newer than the latest. Sealing is intended to be
permanent. A change requires a new draft.`

var (
	errNotNewer    = errors.New("not newer than the latest version")
	errNoVersion   = errors.New("no version, a sealed document needs one")
	errDraftPinned = errors.New("acts on a draft, which has no version")
)

func draftOf(r request) (document, error) {
	arg := r.args[0]
	if strings.Contains(arg, "@") {
		return document{}, fmt.Errorf("%q: %w", arg, errDraftPinned)
	}
	id, err := store.NewID(arg)
	if err != nil {
		return document{}, err
	}
	if err := writable(r.kind, id); err != nil {
		return document{}, err
	}
	scope, err := store.Resolve(r.global)
	if err != nil {
		return document{}, err
	}
	path, err := scope.FindDraft(r.kind, id)
	if err != nil {
		return document{}, err
	}
	return document{kind: r.kind, scope: scope, path: path, at: &store.Entry{Path: path, ID: id, Draft: true}}, nil
}

func sealDocument(r request) (string, string, error) {
	d, err := draftOf(r)
	if err != nil {
		return "", "", err
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
	if err := checkNewer(d, v); err != nil {
		return d.path, "", err
	}
	err = d.scope.Seal(d.kind, d.at.ID, v, data)
	return d.scope.Path(d.kind, d.at.ID, v), sha, err
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
	if err := sealable(d.path, hasSHA(f), err); err != nil {
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
	if err := sealable(d.path, hasSHA(f), err); err != nil {
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

func hasSHA[F *load.TaskFile | *load.WorkflowFile](f F) bool {
	switch f := any(f).(type) {
	case *load.TaskFile:
		return f != nil && f.Doc != nil && f.Doc.SHA != nil
	case *load.WorkflowFile:
		return f != nil && f.Doc != nil && f.Doc.SHA != nil
	}
	return false
}

func sealable(path string, sealed bool, err error) error {
	if sealed {
		return fmt.Errorf("%s: %w, make a new draft to change it", path, seal.ErrSealed)
	}
	return err
}
