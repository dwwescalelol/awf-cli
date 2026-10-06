package render

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/load"
)

func renderBranchy(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "branchy.yaml")
	f, err := load.ReadWorkflow(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Workflow(&out, path, f.Source, f.Doc, f.Compiled, Scope{}, ""); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestWorkflowPage(t *testing.T) {
	out := renderBranchy(t)
	for _, want := range []string{
		`<h1>triage-loop</h1>`,
		`id="task-intake"`,
		`data-task="orphan"`,
		`class="node terminal"`,
		`class="node unreached"`,
		`unreachable from start`,
		`id="mcp-tracker"`,
		`<th>kind</th>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page has no %q", want)
		}
	}
	if strings.Contains(out, "<script>alert") {
		t.Error("task body raw HTML reached the page")
	}
}

func TestFlowOrder(t *testing.T) {
	f, err := load.ReadWorkflow(filepath.Join("testdata", "branchy.yaml"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := f.Compiled
	var names []string
	for _, s := range flowOrder(w) {
		names = append(names, s.Task.Name)
	}
	got := strings.Join(names, " ")
	want := "intake reproduce report answer fix orphan"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLayoutRanks(t *testing.T) {
	f, err := load.ReadWorkflow(filepath.Join("testdata", "branchy.yaml"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := f.Compiled
	l := layout(w)
	ranks := make(map[string]int)
	for _, n := range l.nodes {
		ranks[n.state.Task.Name] = n.rank
	}
	// Back edges do not push a state down, so the retry loops keep fix above report.
	want := map[string]int{"intake": 0, "orphan": 0, "reproduce": 1, "answer": 1, "fix": 2, "report": 3}
	for name, rank := range want {
		if ranks[name] != rank {
			t.Errorf("%s: rank %d, want %d", name, ranks[name], rank)
		}
	}
}
