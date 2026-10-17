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

const importLong = `Copy a document into the ` + store.Local + ` namespace, where it is writable.

The source is a document in any namespace, or a file given by -f. The copy
takes <id> and <version> when given, else the source's own. A workflow's $ref
tasks are bundled into the copy. The copy keeps its sha only while the sha
still matches, so a renamed workflow is unsealed.`

func importKindCmd(kind store.DocumentKind) *cobra.Command {
	cmd := documentCmd(kind, "Copy a "+kind.String()+" into the "+store.Local+" namespace", func(_ *cobra.Command, r request) error {
		i, err := importDocument(r)
		if err != nil {
			return err
		}
		fmt.Println(formatImported(i))
		return nil
	})
	cmd.Use = kind.String() + " ([<namespace>#]<id>[@<version>] | -f <path>) [<id>[@<version>]]"
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
			return fmt.Errorf("accepts one <id>[@<version>] with -f, received %d", len(args))
		case len(args) > n+1:
			return fmt.Errorf("accepts a source %s and one <id>[@<version>], received %d", kind, len(args))
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
	from   string
	sealed bool
}

func formatImported(i imported) string {
	state := "unsealed"
	if i.sealed {
		state = "sealed"
	}
	return fmt.Sprintf("imported %s %s@%s from %s, %s\n%s", i.kind, i.id, i.version, i.from, state, i.path)
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
	if d.at != nil {
		i.from = fmt.Sprintf("%s@%s", d.at.ID, d.at.Version)
	}

	if r.kind == store.Task {
		err = importTask(d, &i, as)
	} else {
		err = importWorkflow(d, &i, as)
	}
	if err != nil {
		return imported{}, err
	}
	i.path = d.scope.Path(i.kind, i.id, i.version)
	return i, nil
}

func importTask(d document, i *imported, as string) error {
	f, err := load.ReadTask(d.path, d.at)
	if err != nil {
		return err
	}
	if err := destination(d, i, as, fileName(d.path), f.Doc.Version); err != nil {
		return err
	}
	i.sealed = f.Doc.SHA != nil
	return scaffold.CopyTask(d.scope, i.id, i.version, f.Doc)
}

func importWorkflow(d document, i *imported, as string) error {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err != nil {
		return err
	}
	if err := destination(d, i, as, f.Doc.Name, f.Doc.Version); err != nil {
		return err
	}
	if err := scaffold.CopyWorkflow(d.scope, i.id, i.version, f.Doc); err != nil {
		return err
	}
	i.sealed = f.Doc.SHA != nil
	return nil
}

// destination sets i's id and version from as, falling back to the source's.
func destination(d document, i *imported, as, name string, v version.Version) error {
	if d.at != nil {
		name = d.at.ID.Name()
	}
	if v == (version.Version{}) {
		v = scaffold.FirstVersion
	}
	if as == "" {
		id, err := store.NewID(name)
		if err != nil {
			return fmt.Errorf("%s: %w: %w", d.path, err, errNoName)
		}
		i.id, i.version = id, v
		return nil
	}
	id, want, pinned, err := parseTarget(as)
	if err != nil {
		return err
	}
	if err := writable(i.kind, id); err != nil {
		return err
	}
	if pinned {
		v = want
	}
	i.id, i.version = id, v
	return nil
}

// fileName is the id a task file names: its directory in a store layout, else its stem.
func fileName(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := version.Parse(stem); err == nil {
		return filepath.Base(filepath.Dir(path))
	}
	return stem
}
