package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

var (
	errIntoScope = errors.New("inside an awf scope")
)

func exportCmd() *cobra.Command {
	return kindCmd(&cobra.Command{
		Use:   "export",
		Short: "Copy a sealed workflow or task to a file outside an awf scope",
	}, exportKindCmd(store.Workflow), exportKindCmd(store.Task))
}

const exportLong = `Copy a sealed document to <path>, a file outside any awf scope.

The source is a document in any namespace, or a file given by -f. Export
refuses a draft, an unsealed file, and a <path> that already exists.`

func exportKindCmd(kind store.DocumentKind) *cobra.Command {
	cmd := documentCmd(kind, "Copy a sealed "+kind.String()+" to a file outside an awf scope", func(_ *cobra.Command, r request) error {
		e, err := exportDocument(r)
		if err != nil {
			return err
		}
		fmt.Println(formatExported(e))
		return nil
	})
	cmd.Use = kind.String() + " ([<namespace>#]<id>[@<version>] | -f <path>) <path>"
	cmd.Long = exportLong
	cmd.Args = destArgs(kind, "<path>", true)
	return cmd
}

type exported struct {
	kind store.DocumentKind
	from string
	path string
}

func formatExported(e exported) string {
	return fmt.Sprintf("exported %s %s\n%s", e.kind, e.from, e.path)
}

func exportDocument(r request) (exported, error) {
	dest := r.args[sourceArgs(r.file)]
	if err := checkKind(dest, r.kind); err != nil {
		return exported{}, err
	}
	if store.InScope(dest) {
		return exported{}, fmt.Errorf("%s: %w, export writes outside one", dest, errIntoScope)
	}
	d, err := resolve(r)
	if err != nil {
		return exported{}, err
	}
	if d.at != nil && d.at.Draft || store.IsDraft(d.path) && store.InScope(d.path) {
		return exported{}, fmt.Errorf("%s: %w, seal it or name a sealed version", d.path, errUnsealed)
	}
	if r.kind == store.Task {
		err = exportTask(d)
	} else {
		err = exportWorkflow(d)
	}
	if err != nil {
		return exported{}, err
	}
	data, err := os.ReadFile(d.path)
	if err != nil {
		return exported{}, err
	}
	err = store.Create(dest, data)
	if errors.Is(err, fs.ErrExist) {
		return exported{}, fmt.Errorf("%s: %w", dest, store.ErrExists)
	}
	if err != nil {
		return exported{}, err
	}
	return exported{kind: r.kind, from: origin(d), path: dest}, nil
}

func exportTask(d document) error {
	f, err := load.ReadTask(d.path, d.at)
	return exportable(d.path, hasSHA(f), err)
}

func exportWorkflow(d document) error {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	return exportable(d.path, hasSHA(f), err)
}

func exportable(path string, sealed bool, err error) error {
	if err != nil {
		return err
	}
	if !sealed {
		return fmt.Errorf("%s: %w, seal it or name a sealed version", path, errUnsealed)
	}
	return nil
}
