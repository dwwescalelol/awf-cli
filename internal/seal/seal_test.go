package seal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/version"
)

func TestCanonical(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{`{"\u20ac":1,"\r":2,"\ufb33":7,"\ud83d\ude00":3,"1":4,"\u0080":5,"\u00f6":6}`, "{\"\\r\":2,\"1\":4,\"\u0080\":5,\"\u00f6\":6,\"\u20ac\":1,\"\U0001F600\":3,\"\ufb33\":7}"},
		{`[1.0, 1e21, 1e-7, 0.000001, -0, 123.456, 1E+2]`, `[1,1e+21,1e-7,0.000001,0,123.456,100]`},
		{`"a\u0001\"\\\n\u2028<>"`, "\"a\\u0001\\\"\\\\\\n\u2028<>\""},
		{`{"a":[true,false,null,{}]}`, `{"a":[true,false,null,{}]}`},
	}
	for _, tt := range tests {
		d := json.NewDecoder(strings.NewReader(tt.in))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			t.Fatal(err)
		}
		got, err := canonical(v)
		if err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		if string(got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.in, got, tt.want)
		}
	}
}

const workflow = `openawf: 0.1.0
name: deploy
version: 0.1.0
sha: null
start: a
orchestration:
  a: null
tasks:
  a:
    version: 0.2.0
    sha: sha256-nested
    body: a
`

const reordered = `tasks:
  a:
    body: a
    sha: sha256-nested
    version: 0.2.0
orchestration:
  a: null
start: a
sha: sha256-whatever
version: 9.9.9
name: deploy
openawf: 0.1.0
`

func workflowSum(t *testing.T, doc string) string {
	t.Helper()
	wf, err := manifest.Unmarshal([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := Workflow(wf)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func TestWorkflow(t *testing.T) {
	base := workflowSum(t, workflow)
	if !strings.HasPrefix(base, "sha256-") || len(base) != len("sha256-")+64 {
		t.Fatalf("got %s, want sha256- and 64 hex digits", base)
	}
	if got := workflowSum(t, reordered); got != base {
		t.Errorf("key order, sha and version: got %s, want %s", got, base)
	}
	for _, edit := range [][2]string{
		{"name: deploy", "name: other"},
		{"body: a", "body: b"},
		{"version: 0.2.0", "version: 0.3.0"},
		{"sha: sha256-nested", "sha: sha256-other"},
	} {
		if got := workflowSum(t, strings.Replace(workflow, edit[0], edit[1], 1)); got == base {
			t.Errorf("%s: hash unchanged", edit[1])
		}
	}
}

func TestTask(t *testing.T) {
	sum := func(doc string) string {
		t.Helper()
		task, err := manifest.UnmarshalTask([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Task(task)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := sum("---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\nmodel: opus\nuses:\n  - gh\n---\n\n# build\n")
	if got := sum("---\nuses:\n  - gh\nmodel: opus\nsha: sha256-x\nversion: 2.0.0\nopenawf: 0.1.0\n---\n# build\n"); got != base {
		t.Errorf("key order, sha and version: got %s, want %s", got, base)
	}
	if got := sum("---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\nmodel: opus\nuses:\n  - gh\n---\n\n# built\n"); got == base {
		t.Error("body edit: hash unchanged")
	}
	if got := sum("---\nopenawf: 0.1.0\nversion: 0.1.0\nsha: null\nmodel: sonnet\nuses:\n  - gh\n---\n\n# build\n"); got == base {
		t.Error("frontmatter edit: hash unchanged")
	}
}

func TestCheck(t *testing.T) {
	wf, err := manifest.Unmarshal([]byte(workflow))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckWorkflow(wf); err != nil {
		t.Errorf("unsealed: got %v", err)
	}
	sum, err := Workflow(wf)
	if err != nil {
		t.Fatal(err)
	}
	wf.SHA = &sum
	if err := CheckWorkflow(wf); err != nil {
		t.Errorf("sealed: got %v", err)
	}
	wf.Name = "other"
	if err := CheckWorkflow(wf); !errors.Is(err, ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, ErrMismatch)
	}
	unknown := "md5-abc"
	wf.SHA = &unknown
	if err := CheckWorkflow(wf); !errors.Is(err, ErrAlgorithm) {
		t.Errorf("md5: got %v, want %v", err, ErrAlgorithm)
	}
}

func workflowContent(t *testing.T, doc string) string {
	t.Helper()
	wf, err := manifest.Unmarshal([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	data, err := manifest.Marshal(wf)
	if err != nil {
		t.Fatal(err)
	}
	got, err := content(data, workflowForm)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

const written = `openawf: 0.1.0
name: keep
summary: ""
metadata:
  a: null
  b: {}
  c: []
  d: ""
version: 1.0.0
sha: sha256-whatever
x-top: {}
x-null: null
x-list: [null, "", {}]
start: a
orchestration:
  a:
    pass: {task: b}
    fail: {task: a, retries: null}
  b:
    task: c
    session: fresh
  c:
    task: d
    x-edge: null
  d: null
tasks:
  a:
    version: 2.0.0
    sha: sha256-nested
    summary: ""
    uses: []
    input:
      type: object
      properties:
        p: {default: null, const: "007", enum: [null, "", "true", 1.50]}
        q: {}
      required: []
    outcomes:
      pass: {}
      fail: {type: object, properties: {}}
    body: a
    x-task: ""
  b:
    output: {}
    body: b
  c:
    body: c
  d:
    body: d
mcp:
  s:
    type: http
    url: https://example.com
    headers: {A: ""}
    oauth: {}
    x-server: []
  t:
    type: stdio
    command: t
    args: []
    env: {}
    deny: []
`

func TestForm(t *testing.T) {
	want := `{"mcp":{"s":{"headers":{"A":""},"type":"http","url":"https://example.com","x-server":[]},"t":{"command":"t","type":"stdio"}},` +
		`"metadata":{"a":null,"b":{},"c":[],"d":""},"name":"keep","openawf":"0.1.0",` +
		`"orchestration":{"a":{"fail":"a","pass":"b"},"b":{"session":"fresh","task":"c"},"c":{"task":"d","x-edge":null},"d":null},` +
		`"start":"a","tasks":{` +
		`"a":{"body":"a","input":{"properties":{"p":{"const":"007","default":null,"enum":[null,"","true",1.5]},"q":{}},"required":[],"type":"object"},` +
		`"outcomes":{"fail":{"properties":{},"type":"object"},"pass":{}},"sha":"sha256-nested","version":"2.0.0","x-task":""},` +
		`"b":{"body":"b"},"c":{"body":"c"},"d":{"body":"d"}},` +
		`"x-list":[null,"",{}],"x-null":null,"x-top":{}}`
	if got := workflowContent(t, written); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	empty := strings.Replace(written, "  a: null\n  b: {}\n  c: []\n  d: \"\"\n", "  {}\n", 1)
	if got := workflowContent(t, empty); strings.Contains(got, "metadata") {
		t.Errorf("empty metadata: got %s", got)
	}
}

func TestEdgeForm(t *testing.T) {
	const doc = "openawf: 0.1.0\nname: e\nstart: a\norchestration:\n  a: EDGE\n  b: null\ntasks:\n  a:\n    body: a\n  b:\n    body: b\n"
	plain := workflowSum(t, strings.Replace(doc, "EDGE", "b", 1))
	if got := workflowSum(t, strings.Replace(doc, "EDGE", "{task: b}", 1)); got != plain {
		t.Errorf("{task: b}: got %s, want %s", got, plain)
	}
	if got := workflowSum(t, strings.Replace(doc, "EDGE", "{task: b, session: fresh}", 1)); got == plain {
		t.Error("session: fresh hashes as no session")
	}
	if got := workflowSum(t, strings.Replace(doc, "EDGE", "{task: b, retries: 0}", 1)); got == plain {
		t.Error("retries: 0 hashes as no retries")
	}
}

func TestAlgorithms(t *testing.T) {
	wf, err := manifest.Unmarshal([]byte(workflow))
	if err != nil {
		t.Fatal(err)
	}
	for name, digits := range map[string]int{"sha256": 64, "sha384": 96, "sha512": 128} {
		sha, err := sumWorkflow(name, wf)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(sha, name+"-") || len(sha) != len(name)+1+digits {
			t.Errorf("%s: got %s", name, sha)
		}
		wf.SHA = &sha
		if err := CheckWorkflow(wf); err != nil {
			t.Errorf("%s: got %v", name, err)
		}
		other := strings.Replace(sha, name, map[string]string{"sha256": "sha512", "sha384": "sha256", "sha512": "sha384"}[name], 1)
		wf.SHA = &other
		if err := CheckWorkflow(wf); !errors.Is(err, ErrMismatch) {
			t.Errorf("%s digest under another prefix: got %v, want %v", name, err, ErrMismatch)
		}
	}
}

func TestCheckInline(t *testing.T) {
	const doc = "---\nopenawf: 0.1.0\nversion: 1.0.0\nsummary: s\n---\n\n# t\n"
	task, err := manifest.UnmarshalTask([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	sha, err := Task(task)
	if err != nil {
		t.Fatal(err)
	}
	inline := *task
	inline.OpenAWF = version.Version{}
	inline.SHA = &sha
	if err := CheckInline(task.OpenAWF, &inline); err != nil {
		t.Errorf("inline: got %v", err)
	}
	inline.Source = "./t.md"
	if err := CheckInline(task.OpenAWF, &inline); err != nil {
		t.Errorf("bundled: got %v", err)
	}
	inline.Summary = "other"
	if err := CheckInline(task.OpenAWF, &inline); !errors.Is(err, ErrMismatch) {
		t.Errorf("edited: got %v, want %v", err, ErrMismatch)
	}
}

func TestGolden(t *testing.T) {
	for file, want := range map[string]string{
		"pr-review.yaml":      "sha256-e6e3e09852b84a72b70656aba73c689deb22c9dfba763805d0ae877f17b0c4a2",
		"feat-dev-small.yaml": "sha256-71bed92b6cddf7e5e894fc6091c7afc2cb164b35349bc6e3622685766b6aedbc",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		if got := workflowSum(t, string(data)); got != want {
			t.Errorf("%s: got %s, want %s", file, got, want)
		}
	}
}

func TestGoldenTasks(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "feat-dev-small.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	wf, err := manifest.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"worktree-setup":  "sha256-4207f150da965878d0fa6273dce20bc86a20de54cac3738dc35a2780be851520",
		"setup-stack":     "sha256-997dfbbefe636e55c494976805866b0a1a640308a3ad3e4b3f92921647aad177",
		"create-diff":     "sha256-34f88c52a41abdbfab4924cdd0a02b6ad08251e2baac49e4a01af1d93e4808fc",
		"testing":         "sha256-3e81d869c79d2a1148cef0dba8f2f9155df2a7ce09e46d03924bb71ab3a4150a",
		"agentic-review":  "sha256-4adb2ca20aa9fcc74fccc9f460e119e254380c1b79c5202586e54add51e505e4",
		"teardown-stack":  "sha256-df55b6b985f45728bb4fbe16b36ef334859c95bd41b337ab6a9b3de7a3e235bf",
		"remove-worktree": "sha256-3eb86c40d716f67fa889976ed348cfd604a59642b059790bbe027dcb80ae4e04",
	} {
		task := *wf.Tasks[name].Task
		task.OpenAWF = wf.OpenAWF
		got, err := Task(&task)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
		task.SHA = &got
		if err := CheckInline(wf.OpenAWF, &task); err != nil {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
