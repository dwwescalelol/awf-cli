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

			kinds := kinds(onlyWorkflows, onlyTasks)
			scope, err := store.Resolve(global)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}

			dir, listings, err := list(scope, kinds)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}
			fmt.Println(formatListing(dir, kinds, listings))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	cmd.Flags().BoolP("wf", "w", false, "list workflows only")
	cmd.Flags().BoolP("task", "t", false, "list tasks only")
	return cmd
}

func kinds(workflows, tasks bool) []store.DocumentKind {
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

func list(scope store.Scope, kinds []store.DocumentKind) (string, map[store.DocumentKind]listing, error) {
	listings := make(map[store.DocumentKind]listing, len(kinds))
	for _, kind := range kinds {
		documents, skipped, err := scope.List(kind)
		if err != nil {
			return "", nil, err
		}
		listings[kind] = listing{documents: documents, skipped: skipped}
	}
	return scope.Dir, listings, nil
}

func formatListing(dir string, kinds []store.DocumentKind, listings map[store.DocumentKind]listing) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, kind := range kinds {
		for _, d := range listings[kind].documents {
			if len(kinds) > 1 {
				fmt.Fprintf(w, "%s\t", kind)
			}
			fmt.Fprintf(w, "%s\t%s\n", d.ID, versions(d))
		}
	}
	w.Flush()

	out := dir + "\nempty"
	if b.Len() > 0 {
		out = dir + "\n" + strings.TrimRight(b.String(), "\n")
	}
	return out + formatSkipped(kinds, listings)
}

func formatSkipped(kinds []store.DocumentKind, listings map[store.DocumentKind]listing) string {
	var b strings.Builder
	for _, kind := range kinds {
		for _, sk := range listings[kind].skipped {
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
