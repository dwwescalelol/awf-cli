// Package version is what this build is: the CLI's own version, the OpenAWF
// version it implements, and the rules both follow.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// CLI is the version awf reports.
const CLI = "0.1.0"

// Spec is the OpenAWF version documents written here declare.
const Spec = "0.1.0"

// First is where a document nobody has written before starts.
const First = "0.1.0"

// Version is a major.minor.patch triple.
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

// NextMinor is the version after v with the patch reset, so the next minor
// after 0.3.1 is 0.4.0.
func (v Version) NextMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1}
}

// Newer reports whether a is a later version than b.
func Newer(a, b Version) bool {
	if a.Major != b.Major {
		return a.Major > b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor > b.Minor
	}
	return a.Patch > b.Patch
}
