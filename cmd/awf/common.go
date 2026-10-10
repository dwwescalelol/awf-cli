package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

var kinds = []store.DocumentKind{store.Workflow, store.Task}

var (
	errUnknownKind = errors.New("invalid document kind")
	errWrongKind   = errors.New("wrong document kind")
	errNoPath      = errors.New("-f requires a <path>")
	errIDAndFile   = errors.New("give an id or -f, not both")
)

const kindsAnnotation = "kinds"

// kindCmd makes cmd take a document kind as its subcommand, one per child.
func kindCmd(cmd *cobra.Command, children ...*cobra.Command) *cobra.Command {
	cmd.Args = kindArgs
	cmd.Annotations = map[string]string{kindsAnnotation: ""}
	cmd.SuggestionsMinimumDistance = 2
	if cmd.RunE == nil {
		cmd.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}
	for _, child := range children {
		if child.Name() == store.Workflow.String() {
			child.SuggestFor = append(child.SuggestFor, "wf")
		}
	}
	cmd.AddCommand(children...)
	return cmd
}

func kindArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	var names []string
	for _, kind := range kinds {
		if child, _, err := cmd.Find([]string{kind.String()}); err == nil && child != cmd {
			names = append(names, kind.String())
		}
	}
	err := fmt.Errorf("%w %q for %q, must be one of [%s]", errUnknownKind, args[0], cmd.CommandPath(), strings.Join(names, ", "))
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		err = fmt.Errorf("%w\n\nDid you mean this?\n\t%s", err, strings.Join(suggestions, "\n\t"))
	}
	return err
}

func targetArgs(kind store.DocumentKind) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		switch {
		case cmd.Flags().Changed("file") && file == "":
			return errNoPath
		case len(args) > 1:
			return fmt.Errorf("accepts one %s <id>[@<version>], received %d", kind, len(args))
		case len(args) == 1 && file != "":
			return errIDAndFile
		case len(args) == 0 && file == "":
			return fmt.Errorf("requires a %s <id>[@<version>]", kind)
		}
		return nil
	}
}

// precheck reports a bad argument at cmd's own level, ahead of help and flag errors.
func precheck(cmd *cobra.Command) error {
	args := cmd.Flags().Args()
	if _, ok := cmd.Annotations[kindsAnnotation]; ok {
		return kindArgs(cmd, args)
	}
	if f := cmd.Flags().Lookup("file"); len(args) > 0 || f != nil && f.Changed {
		return cmd.ValidateArgs(args)
	}
	return nil
}

type request struct {
	kind   store.DocumentKind
	args   []string
	file   string
	global bool
}

// documentCmd runs on one document of kind, named by id or by -f.
func documentCmd(kind store.DocumentKind, short string, run func(*cobra.Command, request) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   kind.String() + " (<id>[@<version>] | -f <path>)",
		Short: short,
		Args:  targetArgs(kind),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			file, _ := cmd.Flags().GetString("file")
			return run(cmd, request{kind: kind, args: args, file: file, global: global})
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a "+kind.String()+" file")
	return cmd
}

type document struct {
	kind  store.DocumentKind
	scope *store.Store
	path  string
	at    *store.Entry
}

func resolve(r request) (document, error) {
	d := document{kind: r.kind, path: r.file}
	if r.file != "" {
		kind, err := store.KindOf(r.file)
		if err != nil {
			return document{}, err
		}
		if kind != r.kind {
			return document{}, fmt.Errorf("%s: a %s, want a %s: %w", r.file, kind, r.kind, errWrongKind)
		}
	}
	var err error
	if d.scope, err = store.Resolve(r.global); err != nil {
		return document{}, err
	}
	if r.file != "" {
		return d, nil
	}
	if d.at, err = locate(d.scope, r.kind, r.args[0]); err != nil {
		return document{}, err
	}
	d.path = d.at.Path
	return d, nil
}

func parseTarget(arg string) (store.ID, version.Version, bool, error) {
	name, pin, pinned := strings.Cut(arg, "@")
	id, err := store.NewID(name)
	if err != nil {
		return "", version.Version{}, false, err
	}
	if !pinned {
		return id, version.Version{}, false, nil
	}
	v, err := version.Parse(pin)
	if err != nil {
		return "", version.Version{}, false, err
	}
	return id, v, true, nil
}

func locate(scope *store.Store, kind store.DocumentKind, arg string) (*store.Entry, error) {
	id, v, pinned, err := parseTarget(arg)
	if err != nil {
		return nil, err
	}
	if !pinned {
		if v, err = scope.Latest(kind, id); err != nil {
			return nil, err
		}
	}
	path, err := scope.Find(kind, id, v)
	if err != nil {
		return nil, err
	}
	return &store.Entry{Path: path, ID: id, Version: v}, nil
}
