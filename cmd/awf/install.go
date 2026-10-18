package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/load"
	"github.com/dwwescalelol/awf-cli/internal/manifest"
	"github.com/dwwescalelol/awf-cli/internal/remote"
	"github.com/dwwescalelol/awf-cli/internal/store"
	"github.com/dwwescalelol/awf-cli/internal/version"
	"github.com/spf13/cobra"
)

var (
	errNoNamespace = errors.New("needs the namespace it is installed from")
	errNoID        = errors.New("which document? add #<id>")
	errAmbiguous   = errors.New("both a workflow and a task")
	errUnbundled   = errors.New("has $ref tasks, ask its author to run awf seal and push")
	errUnsealed    = errors.New("not sealed")
	errConflict    = errors.New("already installed with a different sha")
)

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install (<namespace>#<id>[@<version>] | <url>)",
		Short: "Install a workflow or task from a git repository",
		Long: `Install a sealed workflow or task from a git repository.

The namespace is the repository's address, followed by the path to its .awf
directory when that is not at the root. An unpinned <id> installs its latest
version.

A URL copied from the repository's web page also works: a document's URL, or a
branch or directory's URL followed by #<id>[@<version>]. The branch comes from
the URL unless -b is given.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			branch, _ := cmd.Flags().GetString("branch")
			i, err := install(args[0], branch, global)
			if err != nil {
				return err
			}
			fmt.Println(formatInstalled(i))
			return nil
		},
	}
	cmd.Flags().StringP("branch", "b", "", "install from this git branch, defaults to the repository's default branch")
	return cmd
}

type installation struct {
	kind    store.DocumentKind
	id      store.ID
	version version.Version
	path    string
	existed bool
}

func formatInstalled(i installation) string {
	status := "installed"
	if i.existed {
		status = "already installed"
	}
	return fmt.Sprintf("%s %s %s@%s\n%s", status, i.kind, i.id, i.version, i.path)
}

func install(arg, branch string, global bool) (installation, error) {
	src, ref, err := parseSource(arg)
	if err != nil {
		return installation{}, err
	}
	id, v, pinned, err := parseTarget(ref)
	if err != nil {
		return installation{}, err
	}
	namespace := id.Namespace()
	c, err := src.open(branch)
	if err != nil {
		return installation{}, err
	}
	defer c.Close()

	theirs := store.New(".awf")
	name := store.ID(id.Name())
	kind, available, err := find(c, theirs, name, namespace)
	if err != nil {
		return installation{}, err
	}
	if !pinned {
		v = slices.MaxFunc(available, version.Version.Compare)
	} else if !slices.ContainsFunc(available, func(have version.Version) bool { return have.Compare(v) == 0 }) {
		return installation{}, fmt.Errorf("%s %q version %s in %s: %w, available %s", kind, name, v, namespace, remote.ErrNotFound, versions(store.Document{Versions: available}))
	}
	file := filepath.ToSlash(theirs.Path(kind, name, v))
	data, err := c.Read(file)
	if err != nil {
		return installation{}, err
	}
	at := &store.Entry{ID: id, Version: v}
	sha, err := check(kind, path.Join(namespace, file), data, at)
	if err != nil {
		return installation{}, err
	}

	scope, err := store.Resolve(global)
	if err != nil {
		return installation{}, err
	}
	i := installation{kind: kind, id: id, version: v, path: scope.Path(kind, id, v)}
	existing, err := os.ReadFile(i.path)
	if errors.Is(err, os.ErrNotExist) {
		return i, scope.Write(kind, id, v, data)
	}
	if err != nil {
		return installation{}, err
	}
	if installed, err := check(kind, i.path, existing, at); err != nil || *installed != *sha {
		return installation{}, fmt.Errorf("%s: %w", i.path, errConflict)
	}
	i.existed = true
	return i, nil
}

type source struct {
	namespace string
	repo      string
	dir       string
	branch    string
}

func parseSource(arg string) (source, string, error) {
	target, rest, qualified := strings.Cut(arg, "#")
	repo, branch, file, located := remote.Locate(target)
	if !located {
		if !qualified && !strings.Contains(target, "/") {
			return source{}, "", fmt.Errorf("%q: %w, e.g. github.com/<owner>/<repo>#%s", arg, errNoNamespace, arg)
		}
		if !qualified {
			return source{}, "", fmt.Errorf("%q: %w, e.g. %s#<id>", arg, errNoID, arg)
		}
		namespace := remote.Normalize(target)
		return source{namespace: namespace}, strings.ToLower(namespace) + "#" + rest, nil
	}
	dir, doc, _ := strings.Cut(file+"/", ".awf/")
	if parts := strings.Split(strings.Trim(doc, "/"), "/"); !qualified && len(parts) >= 3 && parts[0] == store.Local {
		rest = parts[2]
		if len(parts) == 4 {
			rest += "@" + strings.TrimSuffix(parts[3], path.Ext(parts[3]))
		}
		qualified = true
	}
	if !qualified {
		return source{}, "", fmt.Errorf("%q: %w, e.g. %s#<id>", arg, errNoID, arg)
	}
	dir = strings.Trim(dir, "/")
	namespace := path.Join(repo, dir)
	return source{namespace: namespace, repo: repo, dir: dir, branch: branch}, strings.ToLower(namespace) + "#" + rest, nil
}

func (s source) open(branch string) (*remote.Checkout, error) {
	if s.repo == "" {
		return remote.Open(s.namespace, branch)
	}
	if branch != "" {
		return remote.Clone(s.repo, s.dir, branch)
	}
	c, err := remote.Clone(s.repo, s.dir, s.branch)
	if errors.Is(err, remote.ErrNoBranch) {
		return nil, fmt.Errorf(`%w, for a branch with "/" in its name use -b`, err)
	}
	return c, err
}

func find(c *remote.Checkout, theirs *store.Store, name store.ID, namespace string) (store.DocumentKind, []version.Version, error) {
	var found []store.DocumentKind
	var versions []version.Version
	for _, kind := range kinds {
		names, err := c.List(filepath.ToSlash(filepath.Dir(theirs.Path(kind, name, version.Version{}))))
		if err != nil {
			return store.DocumentKind{}, nil, err
		}
		var vs []version.Version
		for _, file := range names {
			if k, err := store.KindOf(file); err != nil || k != kind {
				continue
			}
			if v, err := version.Parse(strings.TrimSuffix(file, filepath.Ext(file))); err == nil {
				vs = append(vs, v)
			}
		}
		if len(vs) > 0 {
			found = append(found, kind)
			versions = vs
		}
	}
	switch len(found) {
	case 0:
		return store.DocumentKind{}, nil, fmt.Errorf("%q in %s: %w", name, namespace, remote.ErrNotFound)
	case 1:
		return found[0], versions, nil
	}
	return store.DocumentKind{}, nil, fmt.Errorf("%q in %s: %w", name, namespace, errAmbiguous)
}

func check(kind store.DocumentKind, path string, data []byte, at *store.Entry) (*string, error) {
	var sha *string
	if kind == store.Task {
		f, err := load.ParseTask(path, data, at)
		if err != nil {
			return nil, err
		}
		sha = f.Doc.SHA
	} else {
		if doc, err := manifest.Unmarshal(data); err == nil {
			for _, entry := range doc.Tasks {
				if entry.Task == nil {
					return nil, fmt.Errorf("%s: %w", path, errUnbundled)
				}
			}
		}
		f, err := load.ParseWorkflow(path, data, nil, at, nil)
		if err != nil {
			return nil, err
		}
		sha = f.Doc.SHA
	}
	if sha == nil {
		return nil, fmt.Errorf("%s: %w, ask its author to run awf seal and push", path, errUnsealed)
	}
	return sha, nil
}
