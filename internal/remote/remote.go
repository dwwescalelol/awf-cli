package remote

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
)

var (
	ErrNoGit    = errors.New("git not found on PATH")
	ErrNoRepo   = errors.New("no git repository, or no access to it")
	ErrNoBranch = errors.New("no such branch")
	ErrNotFound = errors.New("not found")
)

func Normalize(addr string) string {
	addr = strings.TrimSpace(addr)
	for _, scheme := range []string{"https://", "http://", "ssh://"} {
		addr = strings.TrimPrefix(addr, scheme)
	}
	if rest, ok := strings.CutPrefix(addr, "git@"); ok {
		addr = strings.Replace(rest, ":", "/", 1)
	}
	return strings.TrimSuffix(strings.TrimSuffix(addr, "/"), ".git")
}

func Locate(page string) (repo, branch, file string, ok bool) {
	if !strings.HasPrefix(page, "https://") && !strings.HasPrefix(page, "http://") {
		return "", "", "", false
	}
	segments := strings.Split(Normalize(page), "/")
	for i := 3; i+1 < len(segments); i++ {
		if !slices.Contains([]string{"blob", "tree", "src"}, segments[i]) {
			continue
		}
		end := i
		if segments[i-1] == "-" {
			end--
		}
		return strings.Join(segments[:end], "/"), segments[i+1], strings.Join(segments[i+2:], "/"), true
	}
	return "", "", "", false
}

type Checkout struct {
	Repo string
	Path string
	dir  string
}

func Open(addr, branch string) (*Checkout, error) {
	segments := strings.Split(addr, "/")
	for i := 2; i <= len(segments); i++ {
		repo := strings.Join(segments[:i], "/")
		if _, err := git("", "ls-remote", "--exit-code", url(repo), "HEAD"); err != nil {
			if errors.Is(err, ErrNoGit) {
				return nil, err
			}
			continue
		}
		return Clone(repo, strings.Join(segments[i:], "/"), branch)
	}
	return nil, fmt.Errorf("%s: %w", addr, ErrNoRepo)
}

func Clone(repo, dir, branch string) (*Checkout, error) {
	args := []string{"clone", "--quiet", "--depth", "1", "--filter=blob:none", "--no-checkout"}
	if branch != "" {
		if _, err := git("", "ls-remote", "--exit-code", url(repo), "refs/heads/"+branch); err != nil {
			if _, err := git("", "ls-remote", url(repo), "HEAD"); err != nil {
				return nil, fmt.Errorf("%s: %w", repo, ErrNoRepo)
			}
			return nil, fmt.Errorf("%s in %s: %w", branch, repo, ErrNoBranch)
		}
		args = append(args, "--branch", branch)
	}
	tmp, err := os.MkdirTemp("", "awf-install-")
	if err != nil {
		return nil, err
	}
	if _, err := git("", append(args, url(repo), tmp)...); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	return &Checkout{Repo: repo, Path: dir, dir: tmp}, nil
}

func (c *Checkout) Close() error { return os.RemoveAll(c.dir) }

func (c *Checkout) List(dir string) ([]string, error) {
	out, err := git(c.dir, "ls-tree", "--name-only", "HEAD", path.Join(c.Path, dir)+"/")
	if err != nil {
		return nil, err
	}
	var names []string
	for line := range strings.Lines(string(out)) {
		names = append(names, path.Base(strings.TrimSpace(line)))
	}
	return names, nil
}

func (c *Checkout) Read(file string) ([]byte, error) {
	object := "HEAD:" + path.Join(c.Path, file)
	if _, err := git(c.dir, "cat-file", "-e", object); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", c.Repo, path.Join(c.Path, file), ErrNotFound)
	}
	return git(c.dir, "show", object)
}

func url(repo string) string { return "https://" + repo }

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, ErrNoGit
	}
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
