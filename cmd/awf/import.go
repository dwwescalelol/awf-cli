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
	cmd.Args = importArgs(kind)
	return cmd
}

// importArgs takes targetArgs' source, then an optional destination.
func importArgs(kind store.DocumentKind) cobra.PositionalArgs {
	source := targetArgs(kind)
	return func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		n := sourceArgs(file)
		switch {
		case len(args) > n+1 && file != "":
			return fmt.Errorf("accepts one <id> with -f, received %d", len(args))
		case len(args) > n+1:
			return fmt.Errorf("accepts a source %s and one <id>, received %d", kind, len(args))
		}
		return source(cmd, args[:min(n, len(args))])
	}
}

// sourceArgs is how many positional arguments name the source: none with -f.
func sourceArgs(file string) int {
	if file != "" {
		return 0
	}
	return 1
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
	i := imported{draft: draft{kind: r.kind}, from: d.path}
	switch {
	case d.at != nil && d.at.Draft:
		i.from = fmt.Sprintf("%s %s", d.at.ID, store.Draft)
	case d.at != nil:
		i.from = fmt.Sprintf("%s@%s", d.at.ID, d.at.Version)
	}

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

// destination sets i's id from as, falling back to the source's name.
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

// fileName is the id a task file names: its directory in a store layout, else its stem.
func fileName(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := version.Parse(stem); err == nil || stem == store.Draft {
		return filepath.Base(filepath.Dir(path))
	}
	return stem
}
