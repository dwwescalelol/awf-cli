package version

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

var ErrNotAVersion = errors.New("not major.minor.patch")

func Parse(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%q: %w", s, ErrNotAVersion)
	}

	var v Version
	into := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("%q: %w", s, ErrNotAVersion)
		}
		*into[i] = n
	}
	return v, nil
}

func MustParse(s string) Version {
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

func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *Version) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*v = Version{}
		return nil
	}
	parsed, err := Parse(string(b))
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
