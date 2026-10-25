// Package schema validates OpenAWF documents against the published JSON Schema.
package schema

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

var (
	ErrNotADocument = errors.New("not a document")
	ErrNoVersion    = errors.New("missing version")
	ErrNotSupported = errors.New("unsupported version")

	ErrOutcomesAndOutput = errors.New("a task has both outcomes and output")
	ErrOutcomeProperty   = errors.New(`an outcome's schema declares a property named "outcome"`)
)

// named maps the location of a schema whose "not" encodes an error of spec
// section 7.1 to that error.
var named = map[string]error{
	"#/$defs/task": ErrOutcomesAndOutput,
	"#/$defs/task/properties/outcomes/additionalProperties": ErrOutcomeProperty,
}

type Violation struct {
	Path []string
	Err  error
}

func (v *Violation) Error() string { return v.Err.Error() }

func (v *Violation) Unwrap() error { return v.Err }

const Spec = "0.1.0"

var SpecVersion = version.MustParse(Spec)

//go:embed schemas/*.json
var schemas embed.FS

func Validate(data []byte) (any, []error) {
	doc, err := decode(data)
	if err != nil {
		return nil, []error{err}
	}
	return doc, check(doc, doc, "")
}

func ValidateTask(data []byte) []error {
	doc, err := decode(data)
	if err != nil {
		return []error{err}
	}
	fields, ok := doc.(map[string]any)
	if !ok {
		return []error{fmt.Errorf("task %T: %w", doc, ErrNotADocument)}
	}
	// A task document declares openawf. The task it holds does not.
	task := maps.Clone(fields)
	delete(task, "openawf")
	return check(doc, task, "#/$defs/task")
}

// check validates doc against the schema at pointer. The spec version comes
// from declared.
func check(declared, doc any, pointer string) []error {
	version, err := versionOf(declared)
	if err != nil {
		return []error{err}
	}
	sch, err := load(version, pointer)
	if err != nil {
		return []error{err}
	}
	err = sch.Validate(doc)
	var invalid *jsonschema.ValidationError
	if errors.As(err, &invalid) {
		return violations(invalid, doc)
	}
	if err != nil {
		return []error{err}
	}
	return nil
}

func violations(e *jsonschema.ValidationError, doc any) []error {
	if errs, ok := explain(e, doc); ok {
		return errs
	}
	causes := e.Causes
	switch e.ErrorKind.(type) {
	case *kind.OneOf, *kind.AnyOf:
		if len(causes) > 0 {
			causes = []*jsonschema.ValidationError{closest(causes)}
		}
	}
	if len(causes) == 0 {
		return leaf(e)
	}
	var out []error
	for _, cause := range causes {
		out = append(out, violations(cause, doc)...)
	}
	return out
}

func leaf(e *jsonschema.ValidationError) []error {
	if _, ok := e.ErrorKind.(*kind.Not); ok {
		_, at, _ := strings.Cut(e.SchemaURL, "#")
		if err, ok := named["#"+at]; ok {
			return []error{located(e.InstanceLocation, err)}
		}
	}
	extra, ok := e.ErrorKind.(*kind.AdditionalProperties)
	if !ok {
		return []error{violation(e.InstanceLocation, e.ErrorKind)}
	}
	out := make([]error, 0, len(extra.Properties))
	for _, name := range extra.Properties {
		path := append(slices.Clone(e.InstanceLocation), name)
		out = append(out, violation(path, &kind.AdditionalProperties{Properties: []string{name}}))
	}
	return out
}

func violation(path []string, k jsonschema.ErrorKind) *Violation {
	msg := (&jsonschema.ValidationError{ErrorKind: k}).BasicOutput().Error.String()
	return located(path, errors.New(msg))
}

func located(path []string, err error) *Violation {
	if len(path) > 0 {
		err = fmt.Errorf("%s: %w", strings.Join(path, "/"), err)
	}
	return &Violation{Path: path, Err: err}
}

func closest(branches []*jsonschema.ValidationError) *jsonschema.ValidationError {
	best, depth, count := branches[0], -1, 0
	for _, b := range branches {
		d, n := extent(b)
		if d > depth || d == depth && n < count {
			best, depth, count = b, d, n
		}
	}
	return best
}

func extent(e *jsonschema.ValidationError) (int, int) {
	if len(e.Causes) == 0 {
		return len(e.InstanceLocation), 1
	}
	depth, count := 0, 0
	for _, cause := range e.Causes {
		d, n := extent(cause)
		depth, count = max(depth, d), count+n
	}
	return depth, count
}

func decode(data []byte) (any, error) {
	raw, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

func versionOf(doc any) (string, error) {
	fields, ok := doc.(map[string]any)
	if !ok {
		return "", fmt.Errorf("document %T: %w", doc, ErrNotADocument)
	}
	version, ok := fields["openawf"].(string)
	if !ok {
		return "", &Violation{Path: []string{"openawf"}, Err: fmt.Errorf("openawf: %w", ErrNoVersion)}
	}
	return version, nil
}

func load(version, pointer string) (*jsonschema.Schema, error) {
	name := "schemas/" + version + ".json"
	file, err := schemas.Open(name)
	if err != nil {
		return nil, &Violation{Path: []string{"openawf"}, Err: fmt.Errorf("openawf %s: %w", version, ErrNotSupported)}
	}
	defer file.Close()
	doc, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		return nil, err
	}
	id := "https://openawf.org/schemas/" + version + "/schema.yaml"

	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		return nil, err
	}
	return c.Compile(id + pointer)
}
