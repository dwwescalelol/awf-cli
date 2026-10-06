package lsp

import (
	"errors"
	"testing"

	"github.com/dwwescalelol/awf-cli/internal/store"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		path string
		text string
		want store.DocumentKind
		err  error
	}{
		{name: "workflow", path: "wf.yaml", text: "openawf: 0.1.0\nname: x\n", want: store.Workflow},
		{name: "yml", path: "wf.yml", text: "name: x\nopenawf: 0.1.0\n", want: store.Workflow},
		{name: "broken workflow", path: "wf.yaml", text: "openawf: 0.1.0\n  : [\n", want: store.Workflow},
		{name: "nested key", path: "wf.yaml", text: "meta:\n  openawf: 0.1.0\n", err: ErrNotOpenAWF},
		{name: "other yaml", path: "ci.yaml", text: "on: push\n", err: ErrNotOpenAWF},
		{name: "task", path: "t.md", text: "---\nopenawf: 0.1.0\n---\n\nbody\n", want: store.Task},
		{name: "crlf task", path: "t.md", text: "---\r\nopenawf: 0.1.0\r\n---\r\n", want: store.Task},
		{name: "unclosed task", path: "t.md", text: "---\nopenawf: 0.1.0\n", want: store.Task},
		{name: "key in body", path: "t.md", text: "---\ntitle: x\n---\nopenawf: 0.1.0\n", err: ErrNotOpenAWF},
		{name: "no frontmatter", path: "t.md", text: "openawf: 0.1.0\n", err: ErrNotOpenAWF},
		{name: "other extension", path: "a.json", text: "openawf: 0.1.0\n", err: ErrNotOpenAWF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.path, []byte(tt.text))
			if !errors.Is(err, tt.err) {
				t.Fatalf("got error %v, want %v", err, tt.err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
