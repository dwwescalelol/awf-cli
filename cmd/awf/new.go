package main

import (
	"fmt"
	"os"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a workflow or task",
	}
	cmd.PersistentFlags().Bool("global", false, "act on ~/.awf")
	cmd.PersistentFlags().StringP("version", "v", "", "version to write, defaults to the next minor")
	cmd.AddCommand(newWorkflowCmd(), newTaskCmd())
	return cmd
}

func newWorkflowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "workflow <id>",
		Short: "Create a workflow, or a new version of one",
		Args:  cobra.ExactArgs(1),
		RunE:  runNew(store.Workflow),
	}
}

func newTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task <id>",
		Short: "Create a task, or a new version of one",
		Args:  cobra.ExactArgs(1),
		RunE:  runNew(store.Task),
	}
}

func runNew(kind store.DocumentKind) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		want, _ := cmd.Flags().GetString("version")

		id, err := store.NewID(args[0])
		if err != nil {
			fmt.Println(formatErr(err))
			return err
		}
		scope, err := store.Resolve(global)
		if err != nil {
			fmt.Println(formatErr(err))
			return err
		}

		path, err := create(scope, kind, id, want)
		if err != nil {
			fmt.Println(formatErr(err))
			return err
		}
		fmt.Println(formatCreated(kind, id, path))
		return nil
	}
}

func formatCreated(kind store.DocumentKind, id store.ID, path string) string {
	return fmt.Sprintf("created %s %s\n%s", kind, id, path)
}

// create writes a draft. A version already in the scope is never overwritten,
// so the path it returns is always a new file. An unnamed version is the first
// one for a new id, and a minor bump for an id already in the scope.
func create(scope store.Scope, kind store.DocumentKind, id store.ID, want string) (string, error) {
	previous, err := scope.Latest(kind, id)
	fresh := err != nil

	v := nextVersion(fresh, previous)
	if want != "" {
		if v, err = version.Parse(want); err != nil {
			return "", err
		}
	}

	path := scope.Path(kind, id, v)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s %q: version %s already exists", kind, id, v)
	}

	var data []byte
	if kind == store.Task {
		data, err = createTask(scope, id, v, fresh, previous)
	} else {
		data, err = createWorkflow(scope, id, v, fresh, previous)
	}
	if err != nil {
		return "", err
	}

	if err := scope.Mkdir(kind, id); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

func nextVersion(fresh bool, previous version.Version) version.Version {
	if fresh {
		return version.FirstVersion
	}
	return previous.NextMinor()
}

func createWorkflow(scope store.Scope, id store.ID, v version.Version, fresh bool, previous version.Version) ([]byte, error) {
	if fresh {
		return manifest.Marshal(manifest.NewWorkflow(id.String(), v))
	}

	data, err := os.ReadFile(scope.Path(store.Workflow, id, previous))
	if err != nil {
		return nil, err
	}
	wf, err := manifest.Parse(data)
	if err != nil {
		return nil, err
	}
	wf.Redraft(v)
	return manifest.Marshal(wf)
}

func createTask(scope store.Scope, id store.ID, v version.Version, fresh bool, previous version.Version) ([]byte, error) {
	if fresh {
		return manifest.MarshalTask(manifest.NewTask(id.String(), v))
	}

	data, err := os.ReadFile(scope.Path(store.Task, id, previous))
	if err != nil {
		return nil, err
	}
	task, err := manifest.ParseTask(data)
	if err != nil {
		return nil, err
	}
	task.Redraft(v)
	return manifest.MarshalTask(task)
}
