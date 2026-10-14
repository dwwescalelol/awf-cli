package main

import (
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestList(t *testing.T) {
	project(t)
	sealed(t, store.Workflow, "feat-dev", "0.1.0")
	sealed(t, store.Workflow, "feat-dev", "0.2.0")
	if _, err := create(store.Workflow, "feat-dev", false); err != nil {
		t.Fatal(err)
	}
	if _, err := create(store.Task, "create-diff", false); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want []store.DocumentKind
	}{
		{name: "both", want: []store.DocumentKind{store.Workflow, store.Task}},
		{name: "workflows", want: []store.DocumentKind{store.Workflow}},
		{name: "tasks", want: []store.DocumentKind{store.Task}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := list(tt.want, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.kinds) != len(tt.want) {
				t.Fatalf("kinds: got %v, want %v", c.kinds, tt.want)
			}
			for i, kind := range tt.want {
				if c.kinds[i] != kind {
					t.Errorf("kinds: got %v, want %v", c.kinds, tt.want)
				}
			}
		})
	}

	c, err := list(kinds, false)
	if err != nil {
		t.Fatal(err)
	}
	wf := c.listings[store.Workflow].documents
	if len(wf) != 1 || wf[0].ID != "feat-dev" || len(wf[0].Versions) != 2 || !wf[0].Draft {
		t.Errorf("workflows: got %+v", wf)
	}
	tasks := c.listings[store.Task].documents
	if len(tasks) != 1 || tasks[0].ID != "create-diff" || len(tasks[0].Versions) != 0 || !tasks[0].Draft {
		t.Errorf("tasks: got %+v", tasks)
	}
}

func TestListEmpty(t *testing.T) {
	project(t)
	c, err := list(kinds, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := formatListing(c); got != c.dir+"\nempty" {
		t.Errorf("got %q", got)
	}
}
