package page

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/render"
	"github.com/dwwescalelol/awf-cli/internal/store"
)

var ErrInvalid = errors.New("does not validate, the page shows why")

func Document(w io.Writer, kind store.DocumentKind, path string, data []byte, s *store.Store, at *store.Entry, scope render.Scope, live string, read load.ReadFunc) error {
	if kind == store.Task {
		return task(w, path, data, at)
	}
	f, err := load.ParseWorkflow(path, data, s, at, read)
	if err != nil {
		return invalid(w, path, render.WorkflowHeader(fileName(path), f.Doc), f.Source, err, scope, live)
	}
	return render.Workflow(w, path, f.Source, f.Doc, f.Compiled, scope, live)
}

func task(w io.Writer, path string, data []byte, at *store.Entry) error {
	name := fileName(path)
	if at != nil {
		name = at.ID.String()
	}
	f, err := load.ParseTask(path, data, at)
	if err != nil {
		return invalid(w, path, render.TaskHeader(name, f.Doc), f.Source, err, render.Scope{}, "")
	}
	return render.Task(w, path, name, f.Source, f.Doc)
}

func invalid(w io.Writer, path string, h render.Header, source []byte, cause error, scope render.Scope, live string) error {
	if err := render.Invalid(w, path, h, source, cause, scope, live); err != nil {
		return err
	}
	return fmt.Errorf("%s: %w", path, ErrInvalid)
}

func fileName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
