package version

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// CLI is the version awf reports. Release builds set it at link time with
// -ldflags "-X github.com/dwwescalelol/awf-cli/internal/version.CLI=<version>".
var CLI = "0.1.0"

const Spec = "0.1.0"

const First = "0.1.0"

var (
	CLIVersion   = mustParse(CLI)
	SpecVersion  = mustParse(Spec)
	FirstVersion = mustParse(First)
)

type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func Parse(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%q: not major.minor.patch", s)
	}

	var v Version
	into := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("%q: not major.minor.patch", s)
		}
		*into[i] = n
	}
	return v, nil
}

func mustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v Version) NextPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

func (v Version) NextMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1}
}

func (v Version) NextMajor() Version {
	return Version{Major: v.Major + 1}
}

func (v Version) Compare(w Version) int {
	return cmp.Or(
		cmp.Compare(v.Major, w.Major),
		cmp.Compare(v.Minor, w.Minor),
		cmp.Compare(v.Patch, w.Patch),
	)
}
