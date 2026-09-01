package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

type listing struct {
	dir  string
	rows []row
}

type row struct {
	kind     store.Kind
	id       string
	versions []string
}

func newLs() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the workflows and tasks in scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l, err := list(global)
			if err != nil {
				return err
			}
			printListing(cmd.OutOrStdout(), l)
			return nil
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "act on ~/.awf")
	return cmd
}

func list(global bool) (listing, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return listing{}, err
	}

	l := listing{dir: scope.Dir}
	for _, kind := range []store.Kind{store.Workflow, store.Task} {
		entries, err := scope.List(kind)
		if err != nil {
			return listing{}, err
		}
		for _, e := range entries {
			l.rows = append(l.rows, row{kind: kind, id: e.ID, versions: e.Versions})
		}
	}
	return l, nil
}

func printListing(out io.Writer, l listing) {
	fmt.Fprintln(out, l.dir)
	if len(l.rows) == 0 {
		fmt.Fprintln(out, "empty")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range l.rows {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.kind, r.id, strings.Join(r.versions, ", "))
	}
	w.Flush()
}
