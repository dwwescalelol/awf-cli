package prompt

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Part
	}{
		{"bare", "{{a}}", []Part{{Field: "a", Offset: 0, End: 5}}},
		{"spaces", "{{ a }}", []Part{{Field: "a", Offset: 0, End: 7}}},
		{"many spaces", "x {{   pull-request  }} y", []Part{
			{Text: "x ", Offset: 0, End: 2},
			{Field: "pull-request", Offset: 2, End: 23},
			{Text: " y", Offset: 23, End: 25},
		}},
		{"triple braces", "{{{a}}}", []Part{
			{Text: "{", Offset: 0, End: 1},
			{Field: "a", Offset: 1, End: 6},
			{Text: "}", Offset: 6, End: 7},
		}},
		{"escaped", `\{{a}}`, []Part{{Text: "{{a}}", Offset: 0, End: 6}}},
		{"escaped backslash", `\\{{a}}`, []Part{
			{Text: `\`, Offset: 0, End: 2},
			{Field: "a", Offset: 2, End: 7},
		}},
		{"lone backslash", `a\b`, []Part{{Text: `a\b`, Offset: 0, End: 3}}},
		{"tab inside", "{{\ta}}", []Part{{Text: "{{\ta}}", Offset: 0, End: 6}}},
		{"tab after", "{{a\t}}", []Part{{Text: "{{a\t}}", Offset: 0, End: 6}}},
		{"inner space", "{{a b}}", []Part{{Text: "{{a b}}", Offset: 0, End: 7}}},
		{"empty", "{{}}", []Part{{Text: "{{}}", Offset: 0, End: 4}}},
		{"unclosed", "{{a}", []Part{{Text: "{{a}", Offset: 0, End: 4}}},
		{"code span", "run `{{cmd}}`", []Part{
			{Text: "run `", Offset: 0, End: 5},
			{Field: "cmd", Offset: 5, End: 12},
			{Text: "`", Offset: 12, End: 13},
		}},
		{"fenced code", "```\n{{a}}\n```\n", []Part{
			{Text: "```\n", Offset: 0, End: 4},
			{Field: "a", Offset: 4, End: 9},
			{Text: "\n```\n", Offset: 9, End: 14},
		}},
		{"adjacent", "{{a}}{{b}}", []Part{
			{Field: "a", Offset: 0, End: 5},
			{Field: "b", Offset: 5, End: 10},
		}},
		{"unicode", "é{{ü}}", []Part{
			{Text: "é", Offset: 0, End: 2},
			{Field: "ü", Offset: 2, End: 8},
		}},
		{"none", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.text, got, tt.want)
			}
		})
	}
}

func TestParams(t *testing.T) {
	got := Params("a {{x}} \\{{y}} {{ z }}")
	if len(got) != 2 || got[0].Field != "x" || got[1].Field != "z" {
		t.Errorf("got %+v, want x and z", got)
	}
}
