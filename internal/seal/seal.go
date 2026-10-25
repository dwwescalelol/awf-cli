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
	"github.com/dwwescalelol/awf-cli/internal/version"
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

// Workflow returns the sha of a bundled workflow.
func Workflow(doc *manifest.Workflow) (string, error) {
	return sumWorkflow(Algorithm, doc)
}

// Task returns the sha of a task document.
func Task(doc *manifest.Task) (string, error) {
	return sumTask(Algorithm, doc)
}

// CheckWorkflow reports whether a sealed workflow's sha matches its content.
func CheckWorkflow(doc *manifest.Workflow) error {
	if doc.SHA == nil {
		return nil
	}
	return check(*doc.SHA, func(name string) (string, error) { return sumWorkflow(name, doc) })
}

// CheckTask reports whether a sealed task document's sha matches its content.
func CheckTask(doc *manifest.Task) error {
	if doc.SHA == nil {
		return nil
	}
	return check(*doc.SHA, func(name string) (string, error) { return sumTask(name, doc) })
}

// CheckInline reports whether a sealed inline task's sha matches its hash as a
// task document carrying the workflow's openawf. A bundled task's sha is its
// hash with source removed.
func CheckInline(openawf version.Version, t *manifest.Task) error {
	standalone := *t
	standalone.OpenAWF = openawf
	err := CheckTask(&standalone)
	if err != nil && standalone.Source != "" {
		standalone.Source = ""
		if CheckTask(&standalone) == nil {
			return nil
		}
	}
	return err
}

func sumWorkflow(name string, doc *manifest.Workflow) (string, error) {
	data, err := manifest.Marshal(doc)
	if err != nil {
		return "", err
	}
	return sum(name, data, workflowForm)
}

func sumTask(name string, doc *manifest.Task) (string, error) {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return sum(name, data, taskForm)
}

// content is the canonical form of section 8.2: the document as a JSON value,
// rewritten by form, serialized with RFC 8785.
func content(data []byte, form func(any) any) ([]byte, error) {
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
	return canonical(form(doc))
}

func check(sha string, sum func(name string) (string, error)) error {
	name, _, _ := strings.Cut(sha, "-")
	got, err := sum(name)
	if err != nil {
		return err
	}
	if got != sha {
		return fmt.Errorf("sha %s, content hashes to %s: %w", sha, got, ErrMismatch)
	}
	return nil
}

func sum(name string, data []byte, form func(any) any) (string, error) {
	newHash, ok := algorithms[name]
	if !ok {
		return "", fmt.Errorf("sha %q: %w", name, ErrAlgorithm)
	}
	body, err := content(data, form)
	if err != nil {
		return "", err
	}
	h := newHash()
	h.Write(body)
	return name + "-" + hex.EncodeToString(h.Sum(nil)), nil
}
