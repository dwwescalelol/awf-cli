package main

import (
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestList(t *testing.T) {
	project(t)
	for _, c := range []struct {
		kind store.DocumentKind
		id   string
	}{{store.Workflow, "feat-dev"}, {store.Workflow, "feat-dev"}, {store.Task, "create-diff"}} {
		if _, err := create(c.kind, c.id, false); err != nil {
			t.Fatal(err)
		}
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
	if len(wf) != 1 || wf[0].ID != "feat-dev" || len(wf[0].Versions) != 2 {
		t.Errorf("workflows: got %+v", wf)
	}
	tasks := c.listings[store.Task].documents
	if len(tasks) != 1 || tasks[0].ID != "create-diff" {
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
