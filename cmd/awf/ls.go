package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func lsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the workflows and tasks in scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			global, _ := cmd.Flags().GetBool("global")
			onlyWorkflows, _ := cmd.Flags().GetBool("wf")
			onlyTasks, _ := cmd.Flags().GetBool("task")

			c, err := list(onlyWorkflows, onlyTasks, global)
			if err != nil {
				return err
			}
			fmt.Println(formatListing(c))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	cmd.Flags().BoolP("wf", "w", false, "list workflows only")
	cmd.Flags().BoolP("task", "t", false, "list tasks only")
	return cmd
}

func selected(workflows, tasks bool) []store.DocumentKind {
	if !workflows && !tasks {
		return []store.DocumentKind{store.Workflow, store.Task}
	}
	kinds := make([]store.DocumentKind, 0, 2)
	if workflows {
		kinds = append(kinds, store.Workflow)
	}
	if tasks {
		kinds = append(kinds, store.Task)
	}
	return kinds
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

func list(onlyWorkflows, onlyTasks, global bool) (contents, error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return contents{}, err
	}
	kinds := selected(onlyWorkflows, onlyTasks)
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
	return out + formatSkipped(c)
}

func formatSkipped(c contents) string {
	var b strings.Builder
	for _, kind := range c.kinds {
		for _, sk := range c.listings[kind].skipped {
			fmt.Fprintf(&b, "\nwarning: %s", sk)
		}
	}
	return b.String()
}

func versions(d store.Document) string {
	out := make([]string, 0, len(d.Versions))
	for _, v := range d.Versions {
		out = append(out, v.String())
	}
	return strings.Join(out, ", ")
}
