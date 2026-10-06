package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/seal"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

func sealCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seal (workflow|task) <id>[@<version>] | seal -f <path>",
		Short: "Write a document's sha after validating it",
		Long:  "Write a document's sha after validating it.\n\n" + sealedNote,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			if file == "" && len(args) == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				return fmt.Errorf("%q: %w", args[0], errUnknownKind)
			}
			return runSeal(cmd, nil, file, false)
		},
	}
	cmd.Flags().StringP("file", "f", "", "path to a document file, a task when it ends in .md")
	cmd.AddCommand(sealWorkflowCmd(), sealTaskCmd())
	return cmd
}

const sealedNote = "Sealing a document is intended to be permanent. A change requires a new version."

func sealWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow <id>[@<version>]",
		Short: "Write a workflow's sha after dereferencing and validating it",
		Long: `Write a workflow's sha after dereferencing and validating it.

Seal refuses a workflow whose $ref tasks are unsealed. --recursive seals them
first. ` + sealedNote,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeal(cmd, args, "", false)
		},
	}
	cmd.Flags().BoolP("recursive", "r", false, "seal the workflow's unsealed $ref tasks first")
	return cmd
}

func sealTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task <id>[@<version>]",
		Short: "Write a task's sha after validating it",
		Long:  "Write a task's sha after validating it.\n\n" + sealedNote,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeal(cmd, args, "", true)
		},
	}
}

func runSeal(cmd *cobra.Command, args []string, file string, task bool) error {
	global, _ := cmd.Flags().GetBool("global")
	recursive, _ := cmd.Flags().GetBool("recursive")

	path, sha, err := sealDocument(args, file, task, recursive, global)
	if err != nil {
		return err
	}
	fmt.Println(path + "\nsealed " + sha)
	return nil
}

func sealDocument(args []string, file string, task, recursive, global bool) (string, string, error) {
	d, err := resolve(args, file, task, global)
	if err != nil {
		return "", "", err
	}
	var data []byte
	var sha string
	if d.kind == store.Task {
		data, sha, err = sealTask(d)
	} else {
		data, sha, err = sealWorkflow(d, recursive)
	}
	if err != nil {
		return d.path, "", err
	}
	if d.at != nil {
		err = d.scope.Replace(d.kind, d.at.ID, d.at.Version, data)
	} else {
		err = os.WriteFile(d.path, data, 0o644)
	}
	return d.path, sha, err
}

func sealTask(d document) ([]byte, string, error) {
	f, err := load.ReadTask(d.path, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", err
	}
	f.Doc.SHA = nil
	sha, err := seal.Task(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.MarshalTask(f.Doc)
	return data, sha, err
}

func sealWorkflow(d document, recursive bool) ([]byte, string, error) {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err := sealable(d.path, f != nil && f.Doc != nil && f.Doc.SHA != nil, err); err != nil {
		return nil, "", err
	}
	if names, paths := unsealed(f); len(names) > 0 {
		if !recursive {
			return nil, "", fmt.Errorf("%s: %w %s, seal them first or use seal workflow --recursive", d.path, ErrUnsealedTasks, strings.Join(names, ", "))
		}
		for _, path := range paths {
			data, _, err := sealTask(document{kind: store.Task, path: path})
			if err != nil {
				return nil, "", err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return nil, "", err
			}
		}
		if f, err = load.ReadWorkflow(d.path, d.scope, d.at); err != nil {
			return nil, "", err
		}
	}
	f.Doc.SHA = nil
	sha, err := seal.Workflow(f.Doc)
	if err != nil {
		return nil, "", err
	}
	f.Doc.SHA = &sha
	data, err := manifest.Marshal(f.Doc)
	return data, sha, err
}

var ErrUnsealedTasks = errors.New("unsealed tasks")

// unsealed names the $ref tasks without a sha, and the files they come from.
func unsealed(f *load.WorkflowFile) ([]string, []string) {
	var names, paths []string
	for name, path := range f.Sources {
		if f.Doc.Tasks[name].Task.SHA != nil {
			continue
		}
		names = append(names, name)
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	slices.Sort(names)
	slices.Sort(paths)
	return names, paths
}

func sealable(path string, sealed bool, err error) error {
	if sealed {
		return fmt.Errorf("%s: %w, make a new version to change it", path, seal.ErrSealed)
	}
	return err
}
