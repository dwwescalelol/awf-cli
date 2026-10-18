package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/scaffold"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	return kindCmd(&cobra.Command{
		Use:   "new",
		Short: "Create a workflow or task",
	}, newKindCmd(store.Workflow), newKindCmd(store.Task))
}

const newLong = `Create the draft of a document in the local namespace.

A draft has no version and no sha. A new draft of an existing id copies its
latest version. Seal gives the draft its version.`

var errNewPinned = errors.New("a draft has no version, seal sets it")

func newKindCmd(kind store.DocumentKind) *cobra.Command {
	return &cobra.Command{
		Use:   kind.String() + " <id>",
		Short: "Create the draft of a " + kind.String(),
		Long:  newLong,
		Args:  cobra.ExactArgs(1),
		RunE:  runNew(kind),
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
	kind store.DocumentKind
	id   store.ID
	path string
}

func formatCreated(d draft) string {
	return fmt.Sprintf("created %s %s draft\n%s", d.kind, d.id, d.path)
}

func create(kind store.DocumentKind, arg string, global bool) (draft, error) {
	name, _, pinned := strings.Cut(arg, "@")
	id, err := store.NewID(name)
	if err != nil {
		return draft{}, err
	}
	if err := writable(kind, id); err != nil {
		return draft{}, err
	}
	if pinned {
		return draft{}, fmt.Errorf("%q: %w", arg, errNewPinned)
	}
	s, err := store.Resolve(global)
	if err != nil {
		return draft{}, err
	}
	if err := scaffold.Create(s, kind, id); err != nil {
		return draft{}, err
	}
	return draft{kind: kind, id: id, path: s.DraftPath(kind, id)}, nil
}
