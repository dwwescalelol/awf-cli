package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestGrammar(t *testing.T) {
	project(t)
	if _, err := create(store.Workflow, "deploy", false); err != nil {
		t.Fatal(err)
	}
	task, err := create(store.Task, "build", false)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args []string
		want error
		text string
	}{
		{args: []string{"seal", "wf", "deploy"}, want: errUnknownKind, text: `invalid document kind "wf" for "awf seal", must be one of [workflow, task]` + "\n\nDid you mean this?\n\tworkflow"},
		{args: []string{"seal", "wf", "-h"}, want: errUnknownKind},
		{args: []string{"seal", "wf", "--bogus"}, want: errUnknownKind},
		{args: []string{"validate", "tsk", "build"}, want: errUnknownKind, text: "Did you mean this?\n\ttask"},
		{args: []string{"bundle", "task", "build"}, want: errUnknownKind, text: "must be one of [workflow]"},
		{args: []string{"ls", "wf"}, want: errUnknownKind},
		{args: []string{"new", "wf", "x"}, want: errUnknownKind},
		{args: []string{"validate", "workflow"}, text: "requires a workflow <id>[@<version>]"},
		{args: []string{"validate", "workflow", "deploy", "extra"}, text: "accepts one workflow <id>[@<version>], received 2"},
		{args: []string{"validate", "workflow", "deploy", "-f", "x.yaml"}, want: errIDAndFile},
		{args: []string{"validate", "workflow", "-f"}, want: errNoPath},
		{args: []string{"validate", "workflow", "-f", ""}, want: errNoPath},
		{args: []string{"validate", "workflow", "-f", "", "-h"}, want: errNoPath},
		{args: []string{"validate", "workflow", "-f", task.path}, want: errWrongKind},
		{args: []string{"validate", "task", "build"}},
		{args: []string{"validate", "workflow", "deploy"}},
		{args: []string{"ls"}},
		{args: []string{"ls", "task"}},
		{args: []string{"seal", "workflow", "-h"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			err := run(tt.args)
			switch {
			case tt.want == nil && tt.text == "" && err != nil:
				t.Errorf("got %v, want no error", err)
			case tt.want != nil && !errors.Is(err, tt.want):
				t.Errorf("got %v, want %v", err, tt.want)
			case tt.text != "" && (err == nil || !strings.Contains(err.Error(), tt.text)):
				t.Errorf("got %v, want %q", err, tt.text)
			}
		})
	}
}
