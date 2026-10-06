package seal

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/goccy/go-yaml"
)

const Algorithm = "sha256"

var (
	ErrMismatch  = errors.New("does not match the content")
	ErrAlgorithm = errors.New("unsupported hash algorithm")
	ErrSealed    = errors.New("already sealed")
)

var algorithms = map[string]func() hash.Hash{
	"sha256": sha256.New,
	"sha384": sha512.New384,
	"sha512": sha512.New,
}

func Workflow(doc *manifest.Workflow) (string, error) {
	data, err := manifest.Marshal(doc)
	if err != nil {
		return "", err
	}
	return sum(Algorithm, data)
}

func Task(doc *manifest.Task) (string, error) {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return sum(Algorithm, data)
}

func CheckWorkflow(doc *manifest.Workflow) error {
	if doc.SHA == nil {
		return nil
	}
	data, err := manifest.Marshal(doc)
	if err != nil {
		return err
	}
	return check(*doc.SHA, data)
}

func CheckTask(doc *manifest.Task) error {
	if doc.SHA == nil {
		return nil
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return check(*doc.SHA, data)
}

func content(data []byte) ([]byte, error) {
	raw, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var doc any
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	if fields, ok := doc.(map[string]any); ok {
		delete(fields, "sha")
		delete(fields, "version")
	}
	return canonical(doc)
}

func check(sha string, data []byte) error {
	name, _, _ := strings.Cut(sha, "-")
	got, err := sum(name, data)
	if err != nil {
		return err
	}
	if got != sha {
		return fmt.Errorf("sha %s, content hashes to %s: %w", sha, got, ErrMismatch)
	}
	return nil
}

func sum(name string, data []byte) (string, error) {
	newHash, ok := algorithms[name]
	if !ok {
		return "", fmt.Errorf("sha %q: %w", name, ErrAlgorithm)
	}
	body, err := content(data)
	if err != nil {
		return "", err
	}
	h := newHash()
	h.Write(body)
	return name + "-" + hex.EncodeToString(h.Sum(nil)), nil
}
