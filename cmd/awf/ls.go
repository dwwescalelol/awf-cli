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

			dir, workflows, tasks, err := list(global)
			if err != nil {
				fmt.Println(formatErr(err))
				return err
			}
			fmt.Println(formatListing(dir, workflows, tasks))
			return nil
		},
	}
	cmd.Flags().Bool("global", false, "act on ~/.awf")
	return cmd
}

func list(global bool) (dir string, workflows, tasks []store.Entry, err error) {
	scope, err := store.Resolve(global)
	if err != nil {
		return "", nil, nil, err
	}
	if workflows, err = scope.List(store.Workflow); err != nil {
		return "", nil, nil, err
	}
	if tasks, err = scope.List(store.Task); err != nil {
		return "", nil, nil, err
	}
	return scope.Dir, workflows, tasks, nil
}

func formatListing(dir string, workflows, tasks []store.Entry) string {
	if len(workflows)+len(tasks) == 0 {
		return dir + "\nempty"
	}

	var b strings.Builder
	b.WriteString(dir + "\n")
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, kind := range []store.Kind{store.Workflow, store.Task} {
		entries := workflows
		if kind == store.Task {
			entries = tasks
		}
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\n", kind, e.ID, strings.Join(e.Versions, ", "))
		}
	}
	w.Flush()
	return strings.TrimRight(b.String(), "\n")
}
