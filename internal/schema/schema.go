// Package schema validates OpenAWF documents against the published JSON Schema.
package schema

import (
	"bytes"
	"embed"
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	ErrNotADocument = errors.New("not a document")
	ErrNoVersion    = errors.New("missing version")
	ErrNotSupported = errors.New("unsupported version")
)

//go:embed schemas/*.json
var schemas embed.FS

func Validate(data []byte) (any, error) {
	doc, err := decode(data)
	if err != nil {
		return nil, err
	}
	version, err := versionOf(doc)
	if err != nil {
		return nil, err
	}
	sch, err := load(version)
	if err != nil {
		return nil, err
	}
	if err := sch.Validate(doc); err != nil {
		return nil, err
	}
	return doc, nil
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
		return "", fmt.Errorf("workflow %T: %w", doc, ErrNotADocument)
	}
	version, ok := fields["openawf"].(string)
	if !ok {
		return "", fmt.Errorf("openawf: %w", ErrNoVersion)
	}
	return version, nil
}

func load(version string) (*jsonschema.Schema, error) {
	name := "schemas/" + version + ".json"
	file, err := schemas.Open(name)
	if err != nil {
		return nil, fmt.Errorf("openawf %s: %w", version, ErrNotSupported)
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
	return c.Compile(id)
}
