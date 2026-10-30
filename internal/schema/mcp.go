package schema

import (
	"errors"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// The MCP server errors of spec section 7.1 that the schema catches.
var (
	ErrServerName        = errors.New("a server name contains /")
	ErrAllowAndDeny      = errors.New("a server has both allow and deny")
	ErrEmptyAllow        = errors.New(`allow is an empty list: list at least one tool, or write "*" for every tool`)
	ErrNotToolList       = errors.New(`allow is neither "*" nor a list of tool names`)
	ErrNoCommand         = errors.New("a stdio server has neither command nor package")
	ErrCommandAndPackage = errors.New("a stdio server has both command and package")
	ErrNoURL             = errors.New("an http or sse server has no url")
	ErrStdioField        = errors.New("a stdio server does not take url, headers or oauth")
	ErrRemoteField       = errors.New("an http or sse server does not take command, package, args or env")
	ErrLatest            = errors.New("a package version is latest, not an exact version")
	ErrNoFileSHA         = errors.New("an mcpb package has no file-sha256")
	ErrFileSHA           = errors.New("file-sha256 is not a SHA-256 in lowercase hex")
)

const (
	server  = "#/$defs/mcpServer"
	stdio   = server + "/allOf/0/then"
	remote  = server + "/allOf/0/else"
	pkg     = "#/$defs/package"
	servers = "#/properties/mcp"
)

var (
	stdioOnly  = []string{"command", "package", "args", "env"}
	remoteOnly = []string{"url", "headers", "oauth"}
)

// explain turns a schema failure on an MCP server into the error of spec
// section 7.1 it encodes. It reports false for a failure it does not cover,
// which then reads as raw schema output.
func explain(e *jsonschema.ValidationError, doc any) ([]error, bool) {
	_, at, _ := strings.Cut(e.SchemaURL, "#")
	at = "#" + at
	path := e.InstanceLocation
	switch k := e.ErrorKind.(type) {
	case *kind.PropertyNames:
		// The library reports this failure at a wrong instance location.
		if at == servers+"/propertyNames" {
			return []error{located([]string{"mcp", k.Property}, ErrServerName)}, true
		}
	case *kind.Not:
		switch at {
		case server:
			return []error{located(append(slices.Clone(path), "deny"), ErrAllowAndDeny)}, true
		case stdio:
			return typed(doc, path, fields(doc, path, remoteOnly), ErrStdioField), true
		case remote:
			return typed(doc, path, fields(doc, path, stdioOnly), ErrRemoteField), true
		case pkg + "/properties/version":
			return []error{located(path, ErrLatest)}, true
		}
	case *kind.OneOf:
		switch at {
		case stdio:
			if len(k.Subschemas) == 0 {
				return typed(doc, path, nil, ErrNoCommand), true
			}
			return typed(doc, path, []string{"package"}, ErrCommandAndPackage), true
		case server + "/properties/allow":
			switch list := instance(doc, path).(type) {
			case []any:
				if len(list) == 0 {
					return []error{located(path, ErrEmptyAllow)}, true
				}
				if len(e.Causes) > 1 {
					return violations(e.Causes[1], doc), true
				}
			}
			return []error{located(path, ErrNotToolList)}, true
		}
	case *kind.Required:
		switch at {
		case remote:
			return typed(doc, path, nil, ErrNoURL), true
		case pkg + "/then":
			return []error{located(path, ErrNoFileSHA)}, true
		}
	case *kind.Pattern:
		if at == pkg+"/properties/file-sha256" {
			return []error{located(path, ErrFileSHA)}, true
		}
	}
	return nil, false
}

// typed reports err on each named field of the server at path, or on the
// server when names is empty. A server whose type is missing or unknown reports
// nothing here: its type is the error, and the schema reports it.
func typed(doc any, path, names []string, err error) []error {
	server, _ := instance(doc, path).(map[string]any)
	switch server["type"] {
	case "stdio", "http", "sse":
	default:
		return nil
	}
	if len(names) == 0 {
		return []error{located(path, err)}
	}
	out := make([]error, 0, len(names))
	for _, name := range names {
		out = append(out, located(append(slices.Clone(path), name), err))
	}
	return out
}

// fields lists the names of candidates the object at path has.
func fields(doc any, path, candidates []string) []string {
	object, _ := instance(doc, path).(map[string]any)
	var out []string
	for _, name := range candidates {
		if _, ok := object[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func instance(doc any, path []string) any {
	for _, segment := range path {
		object, ok := doc.(map[string]any)
		if !ok {
			return nil
		}
		doc = object[segment]
	}
	return doc
}
