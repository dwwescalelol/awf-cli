package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

var update = flag.Bool("update", false, "rewrite the golden pages in testdata")

// TestGolden renders both spec examples and compares each page with its
// golden file. Run with -update to rewrite them.
func TestGolden(t *testing.T) {
	saved := build.Version
	build.Version = "golden"
	t.Cleanup(func() { build.Version = saved })
	for _, name := range []string{"pr-review", "feat-dev-small"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name+".yaml")
			f, err := load.ReadWorkflow(path, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := Workflow(&out, filepath.ToSlash(path), f.Source, f.Doc, f.Compiled, Scope{}, ""); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", name+".golden.html")
			if *update {
				if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Errorf("page differs from %s, rerun with -update and review the diff", golden)
			}
		})
	}
}

func TestSpecFields(t *testing.T) {
	path := filepath.Join("testdata", "feat-dev-small.yaml")
	f, err := load.ReadWorkflow(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Workflow(&out, path, f.Source, f.Doc, f.Compiled, Scope{}, ""); err != nil {
		t.Fatal(err)
	}
	page := out.String()
	for _, want := range []string{
		`<dt>License</dt><dd>Apache-2.0</dd>`,
		`<dt>External version</dt><dd>7c1e4b2a-9d3f-4e6a-b8c0-2f5d1a9e3b47</dd>`,
		`<dt>Authors</dt><dd>OpenAWF</dd>`,
		`<span class="tag">development</span><span class="tag">review</span>`,
		`<dt>Model chain</dt><dd><ol class="chain"><li>anthropic/claude-opus-5-5</li><li>anthropic/claude-sonnet-5</li></ol></dd>`,
		`<h3 class="prose-head">System</h3>`,
		`<h3>Outcome <code>fail</code></h3>`,
		`2 retries, resumes the session`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
}

func TestTaskPage(t *testing.T) {
	var out bytes.Buffer
	doc := &manifest.Task{
		Summary:         "Say hello.",
		License:         "MIT",
		ExternalVersion: "ext-1",
		Metadata:        manifest.Metadata{{Key: "icon", Value: "hello.svg"}, {Key: "authors", Value: []any{"Ada", "Lin"}}},
		Model:           manifest.Models{"anthropic/claude-sonnet-5"},
		Effort:          manifest.Low,
		MaxTurns:        3,
		Timeout:         1500,
		Output:          map[string]any{"type": "string"},
		Body:            "Say hello.",
	}
	if err := Task(&out, "hello.md", "hello", nil, doc); err != nil {
		t.Fatal(err)
	}
	page := out.String()
	for _, want := range []string{
		`<img class="icon" src="hello.svg" alt="">`,
		`<dt>Model</dt><dd><code>anthropic/claude-sonnet-5</code></dd>`,
		`<dt>Effort</dt><dd>low</dd>`,
		`<dt>Max turns</dt><dd>3</dd>`,
		`<dt>Timeout</dt><dd>1500 ms per attempt</dd>`,
		`<dt>Authors</dt><dd>Ada, Lin</dd>`,
		`<dt>External version</dt><dd><code>ext-1</code></dd>`,
		`<h3>Output</h3>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
}

func TestMillis(t *testing.T) {
	for ms, want := range map[int]string{0: "", 1500: "1500 ms", 2000: "2 s", 300000: "5 min", 7200000: "2 h"} {
		if got := millis(ms); got != want {
			t.Errorf("millis(%d) = %q, want %q", ms, got, want)
		}
	}
}

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

func TestServers(t *testing.T) {
	path := filepath.Join("testdata", "pr-review.yaml")
	f, err := load.ReadWorkflow(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Workflow(&out, path, f.Source, f.Doc, f.Compiled, Scope{}, ""); err != nil {
		t.Fatal(err)
	}
	page := out.String()
	for _, want := range []string{
		`<td>http</td><td><code>https://api.githubcopilot.com/mcp/</code></td><td>only get_pull_request, get_pull_request_diff, create_pull_request_review</td>`,
		`<td>stdio</td><td><code>npm @modelcontextprotocol/server-filesystem@2.0.0 ${REPO_DIR}</code></td><td>only read_file, list_directory</td>`,
		`<a class="chip" href="#mcp-github">github/get_pull_request</a>`,
		`<a class="chip" href="#mcp-files">files</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page has no %q", want)
		}
	}
}
