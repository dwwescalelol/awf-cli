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

			dir, entries, err := list(scope, kinds)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}
			fmt.Println(formatListing(dir, kinds, entries))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	cmd.Flags().BoolP("wf", "w", false, "list workflows only")
	cmd.Flags().BoolP("task", "t", false, "list tasks only")
	return cmd
}

func kinds(workflows, tasks bool) []store.Kind {
	if !workflows && !tasks {
		return []store.Kind{store.Workflow, store.Task}
	}
	kinds := make([]store.Kind, 0, 2)
	if workflows {
		kinds = append(kinds, store.Workflow)
	}
	if tasks {
		kinds = append(kinds, store.Task)
	}
	return kinds
}

func list(scope store.Scope, kinds []store.Kind) (string, map[store.Kind][]store.Entry, error) {
	entries := make(map[store.Kind][]store.Entry, len(kinds))
	for _, kind := range kinds {
		found, err := scope.List(kind)
		if err != nil {
			return "", nil, err
		}
		entries[kind] = found
	}
	return scope.Dir, entries, nil
}

func formatListing(dir string, kinds []store.Kind, entries map[store.Kind][]store.Entry) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, kind := range kinds {
		for _, e := range entries[kind] {
			if len(kinds) > 1 {
				fmt.Fprintf(w, "%s\t", kind)
			}
			fmt.Fprintf(w, "%s\t%s\n", e.ID, versions(e))
		}
	}
	w.Flush()

	if b.Len() == 0 {
		return dir + "\nempty"
	}
	return dir + "\n" + strings.TrimRight(b.String(), "\n")
}

func versions(e store.Entry) string {
	out := make([]string, 0, len(e.Versions))
	for _, v := range e.Versions {
		out = append(out, v.String())
	}
	return strings.Join(out, ", ")
}
