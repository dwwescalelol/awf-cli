package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

var errNoName = errors.New("no id to import it as, add <id>")

func importCmd() *cobra.Command {
	return kindCmd(&cobra.Command{
		Use:   "import",
		Short: "Copy a workflow or task into the " + store.Local + " namespace",
	}, importKindCmd(store.Workflow), importKindCmd(store.Task))
}

const importLong = `Copy a document into the draft of <id> in the ` + store.Local + ` namespace, where it is
writable.

The source is a document in any namespace, or a file given by -f. The draft
takes <id> when given, else the source's own. A workflow's $ref tasks are
bundled into the draft. Seal gives the draft its version.`

func importKindCmd(kind store.DocumentKind) *cobra.Command {
	cmd := documentCmd(kind, "Copy a "+kind.String()+" into the "+store.Local+" namespace", func(_ *cobra.Command, r request) error {
		i, err := importDocument(r)
		if err != nil {
			return err
		}
		fmt.Println(formatImported(i))
		return nil
	})
	cmd.Use = kind.String() + " ([<namespace>#]<id>[@<version>] | -f <path>) [<id>]"
	cmd.Long = importLong
	cmd.Args = destArgs(kind, "<id>", false)
	return cmd
}

type imported struct {
	draft
	from string
}

func formatImported(i imported) string {
	return fmt.Sprintf("imported %s %s draft from %s\n%s", i.kind, i.id, i.from, i.path)
}

func importDocument(r request) (imported, error) {
	var as string
	if n := sourceArgs(r.file); len(r.args) > n {
		as = r.args[n]
	}
	d, err := resolve(r)
	if err != nil {
		return imported{}, err
	}
	i := imported{draft: draft{kind: r.kind}, from: origin(d)}

	if r.kind == store.Task {
		err = importTask(d.scope, d, &i, as)
	} else {
		err = importWorkflow(d.scope, d, &i, as)
	}
	if err != nil {
		return imported{}, err
	}
	i.path = d.scope.DraftPath(i.kind, i.id)
	return i, nil
}

func importTask(s *store.Store, d document, i *imported, as string) error {
	f, err := load.ReadTask(d.path, d.at)
	if err != nil {
		return err
	}
	if err := destination(d, i, as, fileName(d.path)); err != nil {
		return err
	}
	return scaffold.CopyTask(s, i.id, f.Doc)
}

func importWorkflow(s *store.Store, d document, i *imported, as string) error {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err != nil {
		return err
	}
	if err := destination(d, i, as, f.Doc.Name); err != nil {
		return err
	}
	return scaffold.CopyWorkflow(s, i.id, f.Doc)
}

func destination(d document, i *imported, as, name string) error {
	if d.at != nil {
		name = d.at.ID.Name()
	}
	if as == "" {
		id, err := store.NewID(name)
		if err != nil {
			return fmt.Errorf("%s: %w: %w", d.path, err, errNoName)
		}
		i.id = id
		return nil
	}
	if strings.Contains(as, "@") {
		return fmt.Errorf("%q: %w", as, errNewPinned)
	}
	id, err := store.NewID(as)
	if err != nil {
		return err
	}
	if err := writable(i.kind, id); err != nil {
		return err
	}
	i.id = id
	return nil
}

func fileName(path string) string {
	name := store.Stem(path)
	if _, err := version.Parse(name); err == nil || store.IsDraft(path) {
		return filepath.Base(filepath.Dir(path))
	}
	return name
}
