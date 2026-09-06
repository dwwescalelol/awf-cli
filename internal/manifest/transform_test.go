package manifest

import "testing"

func TestBuild(t *testing.T) {
	wf := load(t, "testdata/feat-dev-small.yaml")
	if err := wf.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	model := wf.Build()

	if got := model.Start.Task.Name; got != "worktree-setup" {
		t.Errorf("start is %q", got)
	}
	if got := len(model.States); got != 7 {
		t.Errorf("%d states", got)
	}
	if stuck := model.TrapStates(); len(stuck) != 0 {
		t.Errorf("stuck: %v", stuck)
	}

	for _, s := range model.States {
		switch s.Task.Name {
		case "testing":
			if len(s.Out) != 2 {
				t.Fatalf("testing has %d edges", len(s.Out))
			}
			for _, e := range s.Out {
				switch e.On {
				case "pass":
					if e.To.Task.Name != "agentic-review" {
						t.Errorf("pass goes to %q", e.To.Task.Name)
					}
				case "fail":
					if e.To.Task.Name != "create-diff" {
						t.Errorf("fail goes to %q", e.To.Task.Name)
					}
					if e.Retries == nil || *e.Retries != 2 {
						t.Errorf("fail retries %v", e.Retries)
					}
					if e.Session != "resume" {
						t.Errorf("fail session %q", e.Session)
					}
				}
			}
		case "remove-worktree":
			if len(s.Out) != 0 {
				t.Errorf("remove-worktree is not an end")
			}
		}
	}
}
