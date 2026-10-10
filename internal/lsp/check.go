package lsp

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

var (
	ErrNotOpenAWF = errors.New("not an OpenAWF document")
	ErrNotFileURI = errors.New("not a file uri")
)

const (
	severityError   = 1
	severityWarning = 2
)

type site struct {
	target string
	key    string
	rng    textRange
}

func detect(path string, text []byte) (store.DocumentKind, error) {
	kind, err := store.KindOf(path)
	if err == nil && kind == store.Task {
		if _, _, err = manifest.SplitTask(text); errors.Is(err, manifest.ErrOpenFrontmatter) {
			err = nil
		}
	}
	if err != nil || !manifest.Declares(text) {
		return store.DocumentKind{}, fmt.Errorf("%s: %w", path, ErrNotOpenAWF)
	}
	return kind, nil
}

func workflow(path string, text []byte, read load.ReadFunc) (*load.WorkflowFile, []site, error) {
	f, err := load.ParseWorkflow(path, text, scope(path), nil, read)
	lines := splitLines(text)
	var sites []site
	for name, target := range f.Sources {
		line, column := f.Position("tasks", name, "$ref")
		sites = append(sites, site{target: absolute(target), key: canonical(target), rng: lineRange(lines, line, column)})
	}
	return f, sites, err
}

func diagnose(d *document, read load.ReadFunc) []diagnostic {
	kind, err := detect(d.path, d.text)
	if err != nil {
		return nil
	}
	var warnings load.Problems
	if kind == store.Task {
		_, err = load.ParseTask(d.path, d.text, nil)
	} else {
		var f *load.WorkflowFile
		f, d.sites, err = workflow(d.path, d.text, read)
		warnings = f.Warnings
	}
	var problems load.Problems
	errors.As(err, &problems)
	lines := splitLines(d.text)
	here := canonical(d.path)
	var out []diagnostic
	for _, p := range append(problems, warnings...) {
		out = append(out, newDiagnostic(p, lines, here, d.sites, read))
	}
	return out
}

func newDiagnostic(p *load.Problem, lines []string, here string, sites []site, read load.ReadFunc) diagnostic {
	d := diagnostic{Severity: severityError, Source: "awf", Message: p.Message()}
	if p.Warning {
		d.Severity = severityWarning
	}
	file := canonical(p.File)
	if file == here {
		d.Range = lineRange(lines, p.Line, p.Column)
		return d
	}
	for _, s := range sites {
		if s.key == file {
			d.Range = s.rng
		}
	}
	text, _ := read(p.File)
	at := location{URI: fileURI(absolute(p.File)), Range: lineRange(splitLines(text), p.Line, p.Column)}
	d.Related = []related{{Location: at, Message: d.Message}}
	return d
}

func splitLines(text []byte) []string {
	return strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
}

func lineRange(lines []string, line, column int) textRange {
	l := min(max(line-1, 0), len(lines)-1)
	runes := []rune(lines[l])
	start := min(max(column-1, 0), len(runes))
	end := len([]rune(strings.TrimRight(string(runes), " \t")))
	start16 := len(utf16.Encode(runes[:start]))
	end16 := max(len(utf16.Encode(runes[:max(end, start)])), start16)
	return textRange{Start: position{Line: l, Character: start16}, End: position{Line: l, Character: end16}}
}

func scope(path string) *store.Store {
	dir, err := store.FindDirFrom(filepath.Dir(path))
	if err != nil {
		return nil
	}
	return store.New(dir)
}

func absolute(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func canonical(path string) string {
	path = absolute(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

func filePath(uri string) (string, error) {
	path, err := slashPath(uri)
	return filepath.FromSlash(path), err
}

func fileURI(path string) string { return slashURI(filepath.ToSlash(path)) }

func slashPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", fmt.Errorf("%s: %w", uri, ErrNotFileURI)
	}
	if u.Host != "" {
		return "//" + u.Host + u.Path, nil
	}
	if len(u.Path) > 2 && u.Path[2] == ':' {
		return u.Path[1:], nil
	}
	return u.Path, nil
}

func slashURI(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	if rest, ok := strings.CutPrefix(path, "//"); ok {
		host, rest, _ := strings.Cut(rest, "/")
		u.Host, u.Path = host, "/"+rest
	} else if !strings.HasPrefix(path, "/") {
		u.Path = "/" + path
	}
	return u.String()
}
