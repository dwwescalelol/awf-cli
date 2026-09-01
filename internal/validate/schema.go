package validate

import (
	"bytes"
	_ "embed"
	"errors"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed schema.yaml
var schemaYAML []byte

const schemaURL = "https://openawf.org/schemas/0.1.0/schema.yaml"

var printer = message.NewPrinter(language.English)

// schema compiles once. A failure is a broken build, not bad input.
var schema = sync.OnceValue(func() *jsonschema.Schema {
	doc, err := decode(schemaYAML)
	if err != nil {
		panic(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURL, doc); err != nil {
		panic(err)
	}
	return c.MustCompile(schemaURL)
})

// decode routes YAML through JSON so keys are strings and numbers are
// json.Number, which is the shape the validator expects.
func decode(data []byte) (any, error) {
	j, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(j))
}

func structural(d *diags, doc any) {
	err := schema().Validate(doc)
	if err == nil {
		return
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		d.errorf("", "%v", err)
		return
	}
	// The tree repeats a leaf under every branch of a oneOf it failed, so the
	// same finding arrives more than once.
	seen := make(map[Diagnostic]bool)
	for _, diag := range collect(ve, "", nil) {
		if !seen[diag] {
			seen[diag] = true
			*d = append(*d, diag)
		}
	}
}

// collect flattens the error tree to its leaves. A leaf raised by a keyword
// that validates a key rather than a value carries no instance location of its
// own, so it inherits the nearest one above it.
func collect(e *jsonschema.ValidationError, path string, out []Diagnostic) []Diagnostic {
	if p := pointer(e.InstanceLocation); p != "" {
		path = p
	}
	if len(e.Causes) == 0 {
		return append(out, Diagnostic{Path: path, Message: e.ErrorKind.LocalizedString(printer)})
	}
	for _, cause := range e.Causes {
		out = collect(cause, path, out)
	}
	return out
}

func pointer(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return "/" + strings.Join(tokens, "/")
}
