package main

import (
	"errors"
	"fmt"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
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
	cmd.PersistentFlags().Bool("global", false, "act on ~/.awf")
	cmd.PersistentFlags().String("version", "", "version to write, defaults to the next minor")
	cmd.AddCommand(newWorkflowCmd(), newTaskCmd())
	return cmd
}

var errUnknownKind = errors.New("uknown document kind, must be one of [task, workflow]")

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

		id, path, err := create(kind, args[0], want, global)
		if err != nil {
			return err
		}
		fmt.Println(formatCreated(kind, id, path))
		return nil
	}
}

func formatCreated(kind store.DocumentKind, id store.ID, path string) string {
	return fmt.Sprintf("created %s %s\n%s", kind, id, path)
}

func create(kind store.DocumentKind, name, want string, global bool) (store.ID, string, error) {
	id, err := store.NewID(name)
	if err != nil {
		return "", "", err
	}
	s, err := store.Resolve(global)
	if err != nil {
		return "", "", err
	}

	previous, err := s.Latest(kind, id)
	fresh := err != nil

	v := nextVersion(fresh, previous)
	if want != "" {
		if v, err = version.Parse(want); err != nil {
			return "", "", err
		}
	}

	var data []byte
	if kind == store.Task {
		data, err = draftTask(s, id, v, fresh, previous)
	} else {
		data, err = draftWorkflow(s, id, v, fresh, previous)
	}
	if err != nil {
		return "", "", err
	}
	if err := s.Write(kind, id, v, data); err != nil {
		return "", "", err
	}
	return id, s.Path(kind, id, v), nil
}

func nextVersion(fresh bool, previous version.Version) version.Version {
	if fresh {
		return version.FirstVersion
	}
	return previous.NextMinor()
}

func draftWorkflow(s *store.Store, id store.ID, v version.Version, fresh bool, previous version.Version) ([]byte, error) {
	if fresh {
		return manifest.Marshal(blankWorkflow(id.String(), v))
	}
	data, err := s.Read(store.Workflow, id, previous)
	if err != nil {
		return nil, err
	}
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	wf.Version, wf.SHA = v, nil
	return manifest.Marshal(wf)
}

func draftTask(s *store.Store, id store.ID, v version.Version, fresh bool, previous version.Version) ([]byte, error) {
	if fresh {
		return manifest.MarshalTask(blankTask(id.String(), v))
	}
	data, err := s.Read(store.Task, id, previous)
	if err != nil {
		return nil, err
	}
	task, err := manifest.UnmarshalTask(data)
	if err != nil {
		return nil, err
	}
	task.Version, task.SHA = v, nil
	return manifest.MarshalTask(task)
}

func blankWorkflow(id string, v version.Version) *manifest.Workflow {
	const start = "start"
	task := blankTask(start, v)
	task.OpenAWF = version.Version{}
	return &manifest.Workflow{
		OpenAWF:       version.SpecVersion,
		Name:          id,
		Summary:       "What this workflow does.",
		Version:       v,
		Start:         start,
		Orchestration: manifest.Orchestration{start: manifest.Transition{}},
		Tasks:         manifest.Tasks{start: manifest.TaskEntry{Task: task}},
	}
}

func blankTask(id string, v version.Version) *manifest.Task {
	return &manifest.Task{
		OpenAWF: version.SpecVersion,
		Version: v,
		Summary: "What this task does.",
		Body:    fmt.Sprintf("# %s\n\nDescribe the work this task performs, and what it returns.\n", id),
	}
}
