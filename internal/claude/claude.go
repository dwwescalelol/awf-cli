package claude

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
)

type File struct {
	Path string
	Data []byte
}

type pluginJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

type frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Model        string   `yaml:"model,omitempty"`
	AllowedTools []string `yaml:"allowed-tools,omitempty"`
}

func Workflow(name string, wf *manifest.Workflow) ([]File, error) {
	manifestFile, err := plugin(name, wf.Version, wf.Summary)
	if err != nil {
		return nil, err
	}
	skill, err := skillFile(frontmatter{
		Name:         name,
		Description:  describe(wf.Summary, "Run the "+name+" workflow."),
		Model:        wf.Model,
		AllowedTools: allowedTools(wf.MCP),
	}, orchestrator(name, wf))
	if err != nil {
		return nil, err
	}
	dir := path.Join("skills", name)
	files := []File{manifestFile, {Path: path.Join(dir, "SKILL.md"), Data: skill}}
	for _, task := range sortedKeys(wf.Tasks) {
		files = append(files, File{Path: path.Join(dir, "tasks", task+".md"), Data: []byte(taskPage(task, wf.Tasks[task].Task))})
	}
	return files, nil
}

func Task(name string, t *manifest.Task) ([]File, error) {
	manifestFile, err := plugin(name, t.Version, t.Summary)
	if err != nil {
		return nil, err
	}
	skill, err := skillFile(frontmatter{
		Name:        name,
		Description: describe(t.Summary, "Run the "+name+" task."),
		Model:       t.Model,
	}, taskPage(name, t))
	if err != nil {
		return nil, err
	}
	return []File{manifestFile, {Path: path.Join("skills", name, "SKILL.md"), Data: skill}}, nil
}

func plugin(name string, v version.Version, summary string) (File, error) {
	p := pluginJSON{Name: name, Description: summary}
	if v != (version.Version{}) {
		p.Version = v.String()
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return File{}, err
	}
	return File{Path: path.Join(".claude-plugin", "plugin.json"), Data: append(data, '\n')}, nil
}

func skillFile(fm frontmatter, body string) ([]byte, error) {
	head, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + string(head) + "---\n\n" + body), nil
}

func describe(summary, fallback string) string {
	if summary != "" {
		return summary
	}
	return fallback
}

func allowedTools(servers map[string]manifest.MCPServer) []string {
	var tools []string
	for _, name := range sortedKeys(servers) {
		server := strings.NewReplacer(".", "_", " ", "_").Replace(name)
		t := servers[name].Tools
		switch {
		case t == nil || t.All:
			tools = append(tools, "mcp__"+server+"__*")
		default:
			for _, tool := range t.Names {
				tools = append(tools, "mcp__"+server+"__"+tool)
			}
		}
	}
	return tools
}

func orchestrator(name string, wf *manifest.Workflow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	if wf.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", wf.Summary)
	}
	fmt.Fprintf(&b, "Run this workflow as its orchestrator. Start at the task `%s`. ", wf.Start)
	b.WriteString("For each task, follow its file under `tasks/`, then take the row of the orchestration table that matches the outcome the task reports. ")
	b.WriteString("Move to the task in its Next column. Next `end` finishes the workflow. ")
	b.WriteString("Retries caps how many times that row is followed. Session `resume` continues the previous task's session.\n\n")
	b.WriteString("## Orchestration\n\n| Task | Outcome | Next | Retries | Session |\n| --- | --- | --- | --- | --- |\n")
	for _, task := range sortedKeys(wf.Orchestration) {
		t := wf.Orchestration[task]
		switch {
		case t.Edge != nil:
			b.WriteString(row(task, "any", *t.Edge))
		case t.Branch != nil:
			for _, outcome := range sortedKeys(t.Branch) {
				b.WriteString(row(task, outcome, t.Branch[outcome]))
			}
		default:
			fmt.Fprintf(&b, "| `%s` | any | end | | |\n", task)
		}
	}
	b.WriteString("\n## Tasks\n\n")
	for _, task := range sortedKeys(wf.Tasks) {
		fmt.Fprintf(&b, "- [%s](tasks/%s.md)", task, task)
		if s := wf.Tasks[task].Task.Summary; s != "" {
			fmt.Fprintf(&b, ": %s", s)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func row(task, outcome string, e manifest.Edge) string {
	retries := ""
	if e.Retries != nil {
		retries = fmt.Sprint(*e.Retries)
	}
	return fmt.Sprintf("| `%s` | %s | `%s` | %s | %s |\n", task, outcome, e.Task, retries, e.Session)
}

func taskPage(name string, t *manifest.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	if t.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", t.Summary)
	}
	if t.Model != "" {
		fmt.Fprintf(&b, "Model: `%s`\n\n", t.Model)
	}
	if len(t.Outcomes) > 0 {
		fmt.Fprintf(&b, "Report one outcome: %s\n\n", ticks(t.Outcomes))
	}
	if len(t.Uses) > 0 {
		fmt.Fprintf(&b, "MCP servers: %s\n\n", ticks(t.Uses))
	}
	for _, schema := range []struct {
		title string
		value map[string]any
	}{{"Input", t.Input}, {"Output", t.Output}} {
		if len(schema.value) == 0 {
			continue
		}
		data, _ := yaml.Marshal(schema.value)
		fmt.Fprintf(&b, "## %s\n\n```yaml\n%s```\n\n", schema.title, data)
	}
	b.WriteString(strings.TrimSpace(string(t.Body)))
	b.WriteString("\n")
	return b.String()
}

func ticks(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return strings.Join(quoted, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
