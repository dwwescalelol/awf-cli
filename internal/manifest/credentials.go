package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrBadPlaceholder reports a ${...} whose name does not match
// [A-Za-z_][A-Za-z0-9_]*, or that is never closed.
var ErrBadPlaceholder = errors.New("malformed placeholder: write ${NAME}, where NAME matches [A-Za-z_][A-Za-z0-9_]*")

var placeholderName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Placeholders returns the environment variable names that the ${NAME}
// placeholders in s refer to, in the order they appear. Every ${ opens a
// placeholder.
func Placeholders(s string) ([]string, error) {
	var names []string
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			return names, nil
		}
		s = s[start+2:]
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return names, fmt.Errorf("%q: %w", "${"+s, ErrBadPlaceholder)
		}
		name := s[:end]
		if !placeholderName.MatchString(name) {
			return names, fmt.Errorf("%q: %w", "${"+name+"}", ErrBadPlaceholder)
		}
		names = append(names, name)
		s = s[end+1:]
	}
}

// Credentials lists the strings of the server that may hold placeholders,
// those of args, env and headers, each keyed by its path relative to the
// server, such as args/0 or headers/Authorization.
func (s MCPServer) Credentials() []KeyValue {
	var out []KeyValue
	for i, arg := range s.Args {
		out = append(out, KeyValue{Key: fmt.Sprintf("args/%d", i), Value: arg})
	}
	for _, kv := range s.Env {
		out = append(out, KeyValue{Key: "env/" + kv.Key, Value: kv.Value})
	}
	for _, kv := range s.Headers {
		out = append(out, KeyValue{Key: "headers/" + kv.Key, Value: kv.Value})
	}
	return out
}
