// Package validate checks a workflow document against the OpenAWF spec.
package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
)

// supportedVersions is a list because a later CLI reads several spec versions.
var supportedVersions = []string{"0.1.0"}

type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is one finding. Path points into the document in the style of
// manifest.ParseError, and is empty when the finding is on the document itself.
type Diagnostic struct {
	Severity Severity
	Path     string
	Message  string
}

// Result is what validation found. Unresolved names the $ref task entries,
// whose outcomes and uses cannot be checked until the document is bundled.
type Result struct {
	Diagnostics []Diagnostic
	Unresolved  []string
}

func (r Result) Failed() bool {
	return slices.ContainsFunc(r.Diagnostics, func(d Diagnostic) bool {
		return d.Severity == Error
	})
}

// Document runs the structural pass, then the graph pass if the document is
// structurally sound, since the graph pass assumes the shape the schema
// checked. strict promotes warnings to errors.
func Document(data []byte, strict bool) Result {
	var d diags

	doc, err := decode(data)
	if err != nil {
		d.errorf("", "%v", err)
		return d.result(nil, strict)
	}

	// An unsupported version is reported alone: every other check reads the
	// document as the versions this build knows define it.
	if v, ok := declaredVersion(doc); ok && !slices.Contains(supportedVersions, v) {
		d.errorf("/openawf", "openawf %s is not supported, this build reads %s",
			v, strings.Join(supportedVersions, ", "))
		return d.result(nil, strict)
	}

	structural(&d, doc)
	if len(d) > 0 {
		return d.result(nil, strict)
	}

	wf, err := manifest.Parse(data)
	if err != nil {
		d.errorf("", "%v", err)
		return d.result(nil, strict)
	}
	return d.result(graph(&d, wf), strict)
}

func declaredVersion(doc any) (string, bool) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := obj["openawf"].(string)
	return v, ok
}

type diags []Diagnostic

func (d *diags) errorf(path, format string, args ...any) {
	d.add(Error, path, format, args...)
}

func (d *diags) warnf(path, format string, args ...any) {
	d.add(Warning, path, format, args...)
}

func (d *diags) add(s Severity, path, format string, args ...any) {
	*d = append(*d, Diagnostic{Severity: s, Path: path, Message: fmt.Sprintf(format, args...)})
}

func (d diags) result(unresolved []string, strict bool) Result {
	if strict {
		for i := range d {
			d[i].Severity = Error
		}
	}
	slices.SortFunc(d, func(a, b Diagnostic) int {
		if a.Severity != b.Severity {
			return int(a.Severity) - int(b.Severity)
		}
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Message, b.Message)
	})
	return Result{Diagnostics: d, Unresolved: unresolved}
}
