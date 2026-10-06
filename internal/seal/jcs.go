package seal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

var ErrNotJSON = errors.New("not a JSON value")

func canonical(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := write(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func write(b *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case string:
		quote(b, v)
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return err
		}
		return number(b, f)
	case float64:
		return number(b, v)
	case []any:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := write(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b string) int {
			return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
		})
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			quote(b, k)
			b.WriteByte(':')
			if err := write(b, v[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("%T: %w", v, ErrNotJSON)
	}
	return nil
}

func quote(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

func number(b *bytes.Buffer, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("%v: %w", f, ErrNotJSON)
	}
	if f == 0 {
		b.WriteByte('0')
		return nil
	}
	if f < 0 {
		b.WriteByte('-')
		f = -f
	}
	mantissa, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	e, err := strconv.Atoi(exp)
	if err != nil {
		return err
	}
	k, n := len(digits), e+1
	switch {
	case k <= n && n <= 21:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", n-k))
	case 0 < n && n <= 21:
		b.WriteString(digits[:n])
		b.WriteByte('.')
		b.WriteString(digits[n:])
	case -6 < n && n <= 0:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -n))
		b.WriteString(digits)
	default:
		b.WriteString(digits[:1])
		if k > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		if n-1 >= 0 {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(n - 1))
	}
	return nil
}
