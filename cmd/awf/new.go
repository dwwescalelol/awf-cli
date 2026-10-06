package main

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a workflow or task",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("%q: %w", args[0], errUnknownKind)
		},
	}
	cmd.AddCommand(newWorkflowCmd(), newTaskCmd())
	return cmd
}

const newLong = `Create a document, or a new version of one.

The version defaults to the next minor. A new version of an existing id copies
its latest version.`

var errUnknownKind = errors.New("unknown document kind, must be one of [task, workflow]")

func newWorkflowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "workflow <id>[@<version>]",
		Short: "Create a workflow, or a new version of one",
		Long:  newLong,
		Args:  cobra.ExactArgs(1),
		RunE:  runNew(store.Workflow),
	}
}

func newTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task <id>[@<version>]",
		Short: "Create a task, or a new version of one",
		Long:  newLong,
		Args:  cobra.ExactArgs(1),
		RunE:  runNew(store.Task),
	}
}

func runNew(kind store.DocumentKind) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")

		d, err := create(kind, args[0], global)
		if err != nil {
			return err
		}
		fmt.Println(formatCreated(d))
		return nil
	}
}

type draft struct {
	kind    store.DocumentKind
	id      store.ID
	version version.Version
	path    string
}

func formatCreated(d draft) string {
	return fmt.Sprintf("created %s %s@%s\n%s", d.kind, d.id, d.version, d.path)
}

func create(kind store.DocumentKind, arg string, global bool) (draft, error) {
	id, want, pinned, err := parseTarget(arg)
	if err != nil {
		return draft{}, err
	}
	s, err := store.Resolve(global)
	if err != nil {
		return draft{}, err
	}
	v, err := scaffold.Create(s, kind, id, want, pinned)
	if err != nil {
		return draft{}, err
	}
	return draft{kind: kind, id: id, version: v, path: s.Path(kind, id, v)}, nil
}
