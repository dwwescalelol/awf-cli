package remote

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "github.com/zaz/flows", want: "github.com/zaz/flows"},
		{in: "https://github.com/Zaz/Flows.git", want: "github.com/Zaz/Flows"},
		{in: "http://github.com/zaz/flows/", want: "github.com/zaz/flows"},
		{in: "git@github.com:zaz/flows.git", want: "github.com/zaz/flows"},
		{in: "ssh://gitlab.com/acme/ai/flows", want: "gitlab.com/acme/ai/flows"},
		{in: "github.com/acme/mono/x/y", want: "github.com/acme/mono/x/y"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocate(t *testing.T) {
	tests := []struct {
		in                 string
		repo, branch, file string
		ok                 bool
	}{
		{in: "https://github.com/dwwescalelol/awf-cli/blob/main/.awf/_/workflow/feat-dev/0.1.0.yaml", repo: "github.com/dwwescalelol/awf-cli", branch: "main", file: ".awf/_/workflow/feat-dev/0.1.0.yaml", ok: true},
		{in: "https://github.com/dwwescalelol/awf-cli/blob/main", repo: "github.com/dwwescalelol/awf-cli", branch: "main", ok: true},
		{in: "https://github.com/acme/mono/tree/dev/x/y", repo: "github.com/acme/mono", branch: "dev", file: "x/y", ok: true},
		{in: "https://gitlab.com/acme/ai/flows/-/blob/main/.awf/_/task/lint/1.0.0.md", repo: "gitlab.com/acme/ai/flows", branch: "main", file: ".awf/_/task/lint/1.0.0.md", ok: true},
		{in: "https://bitbucket.org/acme/flows/src/main/x", repo: "bitbucket.org/acme/flows", branch: "main", file: "x", ok: true},
		{in: "https://github.com/acme/flows"},
		{in: "github.com/acme/mono/src/flows"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			repo, branch, file, ok := Locate(tt.in)
			if repo != tt.repo || branch != tt.branch || file != tt.file || ok != tt.ok {
				t.Errorf("got %q %q %q %v, want %q %q %q %v", repo, branch, file, ok, tt.repo, tt.branch, tt.file, tt.ok)
			}
		})
	}
}
