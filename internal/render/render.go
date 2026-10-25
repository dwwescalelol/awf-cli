// Package render draws an OpenAWF workflow as a self-contained HTML page.
package render

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
	"github.com/dwwescalelol/awf-cli/internal/build"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed *.html.tmpl
var templates embed.FS

var pages = template.Must(template.New("").Funcs(template.FuncMap{
	"f":  func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) },
	"f0": func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) },
}).ParseFS(templates, "*.html.tmpl"))

// Markdown renders without raw HTML, so a task body cannot inject script.
var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

type Page struct {
	Name     string
	Version  string
	Summary  string
	Model    string
	SHA      string
	Start    string
	Path     string
	Graph    template.HTML
	Tasks    []TaskView
	Servers  []Server
	Warnings []string
	Source   string
	CLI      string
	Scope    Scope
	// Live is the URL of an event stream that fires when the file changes.
	// Empty on a static page.
	Live string
	// Error is why the document does not validate. The page then shows what
	// parsed, under an overlay that reports it.
	Error string
}

// Header is what a document that does not validate still says about itself.
type Header struct {
	Name    string
	Version string
	Summary string
	SHA     string
}

// Scope lists the workflows in the store and the versions of the one on the
// page, so the page can switch to any of them.
type Scope struct {
	Dir       string
	Workflows []Link
	Versions  []Link
}

// Link is one entry of a Scope list.
type Link struct {
	Label   string
	URL     string
	Current bool
	Count   int
	Latest  bool
	Sealed  bool
	Invalid bool
}

type TaskView struct {
	Name        string
	Version     string
	Summary     string
	Model       string
	SHA         string
	Outcomes    []string
	Uses        []Use
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

// Use is one entry of a task's uses: the server, or the qualified tool name,
// that grants the task its tools.
type Use struct {
	Label  string
	Server string
}

type Server struct {
	Name string
	Type string
	// Endpoint is the url of an http or sse server, or what starts a stdio
	// server: its command or package, then its args.
	Endpoint string
	Tools    string
	UsedBy   []string
}

// Workflow writes the page for a compiled workflow. doc supplies the fields
// compiling drops, such as each sha, and source is the file as written.
func Workflow(out io.Writer, path string, source []byte, doc *manifest.Workflow, w *awf.Workflow, scope Scope, live string) error {
	graph, err := SVG(w)
	if err != nil {
		return err
	}
	p := Page{
		Name:    w.Name,
		Version: versionString(w.Version),
		Summary: w.Summary,
		Model:   strings.Join(w.Model, ", "),
		SHA:     sha(doc.SHA),
		Path:    path,
		Graph:   graph,
		Source:  string(source),
		CLI:     build.Version,
		Scope:   scope,
		Live:    live,
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
			var on string
			if e.On != nil {
				on = e.On.Name
			}
			t.Next = append(t.Next, Edge{On: on, To: e.To.Task.Name, Retries: e.Retries, Resume: e.Session == awf.Resume})
		}
		p.Tasks = append(p.Tasks, t)
	}

	for _, srv := range w.Servers {
		s := Server{Name: srv.Name, Type: string(srv.Transport), Endpoint: endpoint(srv), Tools: tools(srv)}
		for _, st := range w.States {
			if slices.ContainsFunc(st.Task.Uses, func(u awf.Use) bool { return u.Server == srv }) {
				s.UsedBy = append(s.UsedBy, st.Task.Name)
			}
		}
		p.Servers = append(p.Servers, s)
	}
	return execute(out, p)
}

// Task writes the page for a stored task. name is its id.
func Task(out io.Writer, path, name string, source []byte, doc *manifest.Task) error {
	outcomes := make([]awf.Outcome, 0, len(doc.Outcomes))
	for _, o := range doc.Outcomes {
		outcomes = append(outcomes, awf.Outcome{Name: o.Name, Schema: o.Schema})
	}
	t, err := task(&awf.Task{
		Name:     name,
		Version:  doc.Version,
		Summary:  doc.Summary,
		Model:    doc.Model,
		Input:    doc.Input,
		Output:   doc.Output,
		Outcomes: outcomes,
		Body:     string(doc.Body),
	}, doc)
	if err != nil {
		return err
	}
	for _, entry := range doc.Uses {
		server, _, _ := strings.Cut(entry, "/")
		t.Uses = append(t.Uses, Use{Label: entry, Server: server})
	}
	return execute(out, Page{
		Name:    name,
		Version: t.Version,
		Summary: doc.Summary,
		SHA:     sha(doc.SHA),
		Path:    path,
		Tasks:   []TaskView{t},
		Source:  string(source),
		CLI:     build.Version,
	})
}

// Invalid writes the page for a document that does not validate: its header
// as far as it parses, its source, and err over the top.
func Invalid(out io.Writer, path string, h Header, source []byte, err error, scope Scope, live string) error {
	return execute(out, Page{
		Name:    h.Name,
		Version: h.Version,
		Summary: h.Summary,
		SHA:     h.SHA,
		Path:    path,
		Source:  string(source),
		CLI:     build.Version,
		Scope:   scope,
		Live:    live,
		Error:   err.Error(),
	})
}

func Message(out io.Writer, title, hint, detail, live string) error {
	return run(out, "message.html.tmpl", struct{ Title, Hint, Detail, Live string }{title, hint, detail, live})
}

func WorkflowHeader(name string, doc *manifest.Workflow) Header {
	h := Header{Name: name}
	if doc == nil {
		return h
	}
	if doc.Name != "" {
		h.Name = doc.Name
	}
	h.Version, h.Summary, h.SHA = versionString(doc.Version), doc.Summary, sha(doc.SHA)
	return h
}

func TaskHeader(name string, doc *manifest.Task) Header {
	h := Header{Name: name}
	if doc != nil {
		h.Version, h.Summary, h.SHA = versionString(doc.Version), doc.Summary, sha(doc.SHA)
	}
	return h
}

func execute(out io.Writer, p Page) error { return run(out, "page.html.tmpl", p) }

func run(out io.Writer, name string, data any) error {
	var buf bytes.Buffer
	if err := pages.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	_, err := buf.WriteTo(out)
	return err
}

func task(t *awf.Task, doc *manifest.Task) (TaskView, error) {
	var body bytes.Buffer
	if err := markdown.Convert([]byte(t.Body), &body); err != nil {
		return TaskView{}, err
	}
	input, err := schemaText(t.Input)
	if err != nil {
		return TaskView{}, err
	}
	output, err := schemaText(t.Output)
	if err != nil {
		return TaskView{}, err
	}
	out := TaskView{
		Name:    t.Name,
		Version: versionString(t.Version),
		Summary: t.Summary,
		Model:   strings.Join(t.Model, ", "),
		Input:   input,
		Output:  output,
		Body:    template.HTML(body.String()),
	}
	if doc != nil {
		out.SHA = sha(doc.SHA)
	}
	for _, o := range t.Outcomes {
		out.Outcomes = append(out.Outcomes, o.Name)
	}
	for _, u := range t.Uses {
		out.Uses = append(out.Uses, Use{Label: u.String(), Server: u.Server.Name})
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

// tools says which of its tools a server lets the workflow call.
func tools(s *awf.MCPServer) string {
	switch {
	case len(s.Deny) > 0:
		return "all except " + strings.Join(s.Deny, ", ")
	case s.Allow.All:
		return "all tools"
	case len(s.Allow.Names) == 0:
		return "no tools"
	}
	return "only " + strings.Join(s.Allow.Names, ", ")
}

// endpoint is the url of an http or sse server, or the command or package
// that starts a stdio server followed by its args.
func endpoint(s *awf.MCPServer) string {
	if s.URL != "" {
		return s.URL
	}
	start := s.Command
	if p := s.Package; p != nil {
		start = packageText(p)
	}
	return strings.Join(append([]string{start}, s.Args...), " ")
}

// packageText names a package as its registry does: npm name@version, pypi
// name==version, an oci image reference, or an mcpb bundle URL with its
// version.
func packageText(p *awf.Package) string {
	switch p.RegistryType {
	case "npm":
		return "npm " + p.Identifier + "@" + p.Version
	case "pypi":
		return "pypi " + p.Identifier + "==" + p.Version
	case "oci":
		return "oci " + p.Identifier + ":" + p.Version
	}
	return p.RegistryType + " " + p.Identifier + " " + p.Version
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
