package lsp

import (
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/ref"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type site struct {
	rng    Range
	target string
}

func refSites(path string, text []byte, s *store.Store) []site {
	f, err := parser.ParseBytes(text, 0)
	if err != nil || len(f.Docs) == 0 {
		return nil
	}
	lines := splitLines(text)
	dir := load.Base(path, s)
	var sites []site
	for _, entry := range mapping(lookup(f.Docs[0].Body, "tasks")) {
		node, ok := unwrap(lookup(entry.Value, "$ref")).(*ast.StringNode)
		if !ok {
			continue
		}
		target, err := locate(node.Value, dir, s)
		if err != nil {
			continue
		}
		tk := node.GetToken()
		sites = append(sites, site{rng: lineRange(lines, tk.Position.Line, tk.Position.Column), target: target})
	}
	return sites
}

func locate(raw, dir string, s *store.Store) (string, error) {
	r, err := ref.Parse(raw)
	if err != nil {
		return "", err
	}
	path, err := r.Locate(dir, s)
	if err != nil {
		return "", err
	}
	return canonical(path), nil
}

func lookup(node ast.Node, key string) ast.Node {
	for _, entry := range mapping(node) {
		if s, ok := unwrap(entry.Key).(*ast.StringNode); ok && s.Value == key {
			return entry.Value
		}
	}
	return nil
}

func mapping(node ast.Node) []*ast.MappingValueNode {
	switch n := unwrap(node).(type) {
	case *ast.MappingNode:
		return n.Values
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{n}
	}
	return nil
}

func unwrap(node ast.Node) ast.Node {
	for {
		switch n := node.(type) {
		case *ast.AnchorNode:
			node = n.Value
		case *ast.TagNode:
			node = n.Value
		default:
			return node
		}
	}
}

func canonical(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path
}

func (r Range) contains(p Position) bool {
	return p.Line == r.Start.Line && p.Character >= r.Start.Character && p.Character <= r.End.Character
}
