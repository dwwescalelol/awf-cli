package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dwwescalelol/awf-cli/internal/claude"
	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/spf13/cobra"
)

var (
	errIntoScope = errors.New("inside an awf scope")
	errFormat    = errors.New("unknown export format")
)

const (
	formatAWF    = "awf"
	formatClaude = "claude"
)

func exportCmd() *cobra.Command {
	return kindCmd(&cobra.Command{
		Use:   "export",
		Short: "Copy a sealed workflow or task to a file outside an awf scope",
	}, exportKindCmd(store.Workflow), exportKindCmd(store.Task))
}

const exportLong = `Copy a sealed document to <path>, a file outside any awf scope.

The source is a document in any namespace, or a file given by -f. Export
refuses a draft, an unsealed file, and a <path> that already exists.

--format awf, the default, copies the document to the file <path>. --format
claude writes a Claude Code plugin to the directory <path>: a skill that
orchestrates a workflow, or runs a task, with one file per task.`

func exportKindCmd(kind store.DocumentKind) *cobra.Command {
	cmd := documentCmd(kind, "Copy a sealed "+kind.String()+" to a file outside an awf scope", func(cmd *cobra.Command, r request) error {
		format, _ := cmd.Flags().GetString("format")
		e, err := exportDocument(r, format)
		if err != nil {
			return err
		}
		fmt.Println(formatExported(e))
		return nil
	})
	cmd.Use = kind.String() + " ([<namespace>#]<id>[@<version>] | -f <path>) <path>"
	cmd.Long = exportLong
	cmd.Args = destArgs(kind, "<path>", true)
	cmd.Flags().String("format", formatAWF, "export format: "+formatAWF+" or "+formatClaude)
	return cmd
}

type exported struct {
	kind store.DocumentKind
	from string
	path string
}

func formatExported(e exported) string {
	return fmt.Sprintf("exported %s %s\n%s", e.kind, e.from, e.path)
}

func exportDocument(r request, format string) (exported, error) {
	dest := r.args[sourceArgs(r.file)]
	switch format {
	case formatAWF:
		if err := checkKind(dest, r.kind); err != nil {
			return exported{}, err
		}
	case formatClaude:
	default:
		return exported{}, fmt.Errorf("%q: %w, want %s or %s", format, errFormat, formatAWF, formatClaude)
	}
	if format == formatAWF && store.InScope(dest) {
		return exported{}, fmt.Errorf("%s: %w, export writes outside one", dest, errIntoScope)
	}
	d, err := resolve(r)
	if err != nil {
		return exported{}, err
	}
	if d.at != nil && d.at.Draft || store.IsDraft(d.path) && store.InScope(d.path) {
		return exported{}, fmt.Errorf("%s: %w, seal it or name a sealed version", d.path, errUnsealed)
	}
	var files []claude.File
	if r.kind == store.Task {
		files, err = exportTask(d, format)
	} else {
		files, err = exportWorkflow(d, format)
	}
	if err != nil {
		return exported{}, err
	}
	if format == formatAWF {
		err = write(dest, d.path)
	} else {
		err = writeDir(dest, files)
	}
	if err != nil {
		return exported{}, err
	}
	return exported{kind: r.kind, from: origin(d), path: dest}, nil
}

func exportTask(d document, format string) ([]claude.File, error) {
	f, err := load.ReadTask(d.path, d.at)
	if err := exportable(d.path, hasSHA(f), err); err != nil || format == formatAWF {
		return nil, err
	}
	return claude.Task(exportName(d), f.Doc)
}

func exportWorkflow(d document, format string) ([]claude.File, error) {
	f, err := load.ReadWorkflow(d.path, d.scope, d.at)
	if err := exportable(d.path, hasSHA(f), err); err != nil || format == formatAWF {
		return nil, err
	}
	return claude.Workflow(exportName(d), f.Doc)
}

func exportName(d document) string {
	if d.at != nil {
		return d.at.ID.Name()
	}
	return fileName(d.path)
}

func exportable(path string, sealed bool, err error) error {
	if err != nil {
		return err
	}
	if !sealed {
		return fmt.Errorf("%s: %w, seal it or name a sealed version", path, errUnsealed)
	}
	return nil
}

func write(dest, source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	err = store.Create(dest, data)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", dest, store.ErrExists)
	}
	return err
}

func writeDir(dest string, files []claude.File) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("%s: %w", dest, store.ErrExists)
	}
	for _, f := range files {
		if store.InScope(filepath.Join(dest, filepath.FromSlash(f.Path))) {
			return fmt.Errorf("%s: %w, export writes outside one", dest, errIntoScope)
		}
	}
	for _, f := range files {
		if err := store.Create(filepath.Join(dest, filepath.FromSlash(f.Path)), f.Data); err != nil {
			return err
		}
	}
	return nil
}
