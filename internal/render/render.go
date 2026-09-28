// Package render draws an OpenAWF document as a self-contained HTML page.
package render

import (
	"bytes"
	_ "embed"
	"html/template"
	"io"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed page.html.tmpl
var pageTemplate string

var page = template.Must(template.New("page").Parse(pageTemplate))

// Markdown renders without raw HTML, so a task body cannot inject script.
var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

type Page struct {
	Kind     string
	Name     string
	Version  string
	Summary  string
	Model    string
	SHA      string
	Start    string
	Path     string
	Graph    template.HTML
	Tasks    []Task
	Servers  []Server
	Warnings []string
	Source   string
	CLI      string
}

type Task struct {
	Name        string
	Version     string
	Summary     string
	Model       string
	SHA         string
	Outcomes    []string
	Uses        []string
	Input       string
	Output      string
	Body        template.HTML
	Next        []Edge
	Start       bool
	Terminal    bool
	Unreachable bool
}

type Edge struct {
	On      string
	To      string
	Retries *int
	Resume  bool
}

type Server struct {
	Name      string
	Transport string
	Tools     string
	UsedBy    []string
}

// Workflow writes the page for a compiled workflow. doc supplies the fields
// compiling drops, such as each sha, and source is the file as written.
func Workflow(out io.Writer, path string, source []byte, doc *manifest.Workflow, w *awf.Workflow) error {
	p := Page{
		Kind:    "workflow",
		Name:    w.Name,
		Version: versionString(w.Version),
		Summary: w.Summary,
		Model:   w.Model,
		SHA:     sha(doc.SHA),
		Path:    path,
		Graph:   SVG(w),
		Source:  string(source),
		CLI:     version.CLI,
	}
	if w.Start != nil {
		p.Start = w.Start.Task.Name
	}
	for _, warning := range w.Warnings() {
		p.Warnings = append(p.Warnings, warning.Error())
	}

	unreachable := make(map[*awf.State]bool)
	for _, s := range w.Unreachable() {
		unreachable[s] = true
	}
	for _, s := range flowOrder(w) {
		t, err := task(s.Task, doc.Tasks[s.Task.Name].Task)
		if err != nil {
			return err
		}
		t.Start = s == w.Start
		t.Terminal = s.IsTerminal()
		t.Unreachable = unreachable[s]
		for _, e := range s.Out {
			t.Next = append(t.Next, Edge{On: string(e.On), To: e.To.Task.Name, Retries: e.Retries, Resume: e.Session == awf.Resume})
		}
		p.Tasks = append(p.Tasks, t)
	}

	for _, srv := range w.Servers {
		s := Server{Name: srv.Name, Transport: string(srv.Transport), Tools: tools(srv.Tools)}
		for _, st := range w.States {
			if slices.Contains(st.Task.Uses, srv) {
				s.UsedBy = append(s.UsedBy, st.Task.Name)
			}
		}
		p.Servers = append(p.Servers, s)
	}
	return execute(out, p)
}

// TaskPage writes the page for a standalone task file.
func TaskPage(out io.Writer, path, name string, source []byte, doc *manifest.Task) error {
	outcomes := make([]awf.Outcome, 0, len(doc.Outcomes))
	for _, o := range doc.Outcomes {
		outcomes = append(outcomes, awf.Outcome(o))
	}
	t, err := task(&awf.Task{
		Name:     name,
		Version:  doc.Version,
		Summary:  doc.Summary,
		Model:    doc.Model,
		Input:    doc.Input,
		Output:   doc.Output,
		Outcomes: outcomes,
		Body:     doc.Body,
	}, doc)
	if err != nil {
		return err
	}
	t.Uses = doc.Uses
	return execute(out, Page{
		Kind:    "task",
		Name:    name,
		Version: t.Version,
		Summary: doc.Summary,
		Model:   doc.Model,
		SHA:     sha(doc.SHA),
		Path:    path,
		Tasks:   []Task{t},
		Source:  string(source),
		CLI:     version.CLI,
	})
}

func execute(out io.Writer, p Page) error {
	var buf bytes.Buffer
	if err := page.Execute(&buf, p); err != nil {
		return err
	}
	_, err := buf.WriteTo(out)
	return err
}

func task(t *awf.Task, doc *manifest.Task) (Task, error) {
	var body bytes.Buffer
	if err := markdown.Convert([]byte(t.Body), &body); err != nil {
		return Task{}, err
	}
	input, err := schemaText(t.Input)
	if err != nil {
		return Task{}, err
	}
	output, err := schemaText(t.Output)
	if err != nil {
		return Task{}, err
	}
	out := Task{
		Name:    t.Name,
		Version: versionString(t.Version),
		Summary: t.Summary,
		Model:   t.Model,
		Input:   input,
		Output:  output,
		Body:    template.HTML(body.String()),
	}
	if doc != nil {
		out.SHA = sha(doc.SHA)
	}
	for _, o := range t.Outcomes {
		out.Outcomes = append(out.Outcomes, string(o))
	}
	for _, u := range t.Uses {
		out.Uses = append(out.Uses, u.Name)
	}
	return out, nil
}

// flowOrder lists states breadth-first from the start, so tasks read in the
// order a run meets them. States the start never reaches follow by name.
func flowOrder(w *awf.Workflow) []*awf.State {
	seen := make(map[*awf.State]bool, len(w.States))
	var out []*awf.State
	if w.Start != nil {
		queue := []*awf.State{w.Start}
		seen[w.Start] = true
		for len(queue) > 0 {
			s := queue[0]
			queue = queue[1:]
			out = append(out, s)
			for _, e := range s.Out {
				if !seen[e.To] {
					seen[e.To] = true
					queue = append(queue, e.To)
				}
			}
		}
	}
	for _, s := range w.States {
		if !seen[s] {
			out = append(out, s)
		}
	}
	return out
}

func schemaText(schema map[string]any) (string, error) {
	if len(schema) == 0 {
		return "", nil
	}
	data, err := yaml.MarshalWithOptions(schema, yaml.IndentSequence(true))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func tools(t awf.Tools) string {
	if t.All {
		return "all tools"
	}
	if len(t.Names) == 0 {
		return "no tools"
	}
	return strings.Join(t.Names, ", ")
}

func versionString(v version.Version) string {
	if v == (version.Version{}) {
		return ""
	}
	return v.String()
}

func sha(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
