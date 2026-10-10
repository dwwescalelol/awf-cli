package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func lsCmd() *cobra.Command {
	children := make([]*cobra.Command, len(kinds))
	for i, kind := range kinds {
		children[i] = &cobra.Command{
			Use:   kind.String(),
			Short: "List the " + kind.String() + "s in scope",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return runList(cmd, []store.DocumentKind{kind})
			},
		}
	}
	return kindCmd(&cobra.Command{
		Use:   "ls",
		Short: "List the workflows and tasks in scope",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd, kinds)
		},
	}, children...)
}

func runList(cmd *cobra.Command, kinds []store.DocumentKind) error {
	global, _ := cmd.Flags().GetBool("global")
	c, err := list(kinds, global)
	if err != nil {
		return err
	}
	fmt.Println(formatListing(c))
	for _, kind := range c.kinds {
		for _, sk := range c.listings[kind].skipped {
			warn(sk.String())
		}
	}
	return nil
}

type listing struct {
	documents []store.Document
	skipped   []store.Skipped
}

type contents struct {
	dir      string
	kinds    []store.DocumentKind
	listings map[store.DocumentKind]listing
}

func list(kinds []store.DocumentKind, global bool) (contents, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return contents{}, err
	}
	listings := make(map[store.DocumentKind]listing, len(kinds))
	for _, kind := range kinds {
		documents, skipped, err := scope.List(kind)
		if err != nil {
			return contents{}, err
		}
		listings[kind] = listing{documents: documents, skipped: skipped}
	}
	return contents{dir: scope.Dir(), kinds: kinds, listings: listings}, nil
}

func formatListing(c contents) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, kind := range c.kinds {
		for _, d := range c.listings[kind].documents {
			if len(c.kinds) > 1 {
				fmt.Fprintf(w, "%s\t", kind)
			}
			fmt.Fprintf(w, "%s\t%s\n", d.ID, versions(d))
		}
	}
	w.Flush()

	out := c.dir + "\nempty"
	if b.Len() > 0 {
		out = c.dir + "\n" + strings.TrimRight(b.String(), "\n")
	}
	return out
}

func versions(d store.Document) string {
	out := make([]string, 0, len(d.Versions))
	for _, v := range d.Versions {
		out = append(out, v.String())
	}
	return strings.Join(out, ", ")
}
