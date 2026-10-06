package seal

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
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
    outcomes: []
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
