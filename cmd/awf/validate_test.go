package main

import (
	"strings"
	"testing"
)

func TestValidateFileNoTerminal(t *testing.T) {
	_, err := validateFile("testdata/no-end.yaml", false)
	if err == nil {
		t.Fatal("got nil, want an error")
	}

	want := "invalid /orchestration: no task ends the flow, so it can never finish"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("%q not reported, got:\n%v", want, err)
	}
}

func TestValidateFileTrap(t *testing.T) {
	_, err := validateFile("testdata/trap.yaml", false)
	if err == nil {
		t.Fatal("got nil, want an error")
	}

	for _, name := range []string{"repair", "recheck"} {
		want := "invalid /orchestration/" + name + ": the flow can never leave this task"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q not reported, got:\n%v", want, err)
		}
	}
	for _, name := range []string{"build", "done"} {
		if strings.Contains(err.Error(), "/orchestration/"+name+":") {
			t.Errorf("%q reported, got:\n%v", name, err)
		}
	}
}

func TestValidateFileEnds(t *testing.T) {
	path, err := validateFile("testdata/ends.yaml", false)
	if err != nil {
		t.Fatalf("got %v, want nil", err)
	}
	if path != "testdata/ends.yaml" {
		t.Errorf("got %q, want the file checked", path)
	}
}

func TestFormatValid(t *testing.T) {
	got := formatValid("testdata/ends.yaml")
	want := "testdata/ends.yaml\nvalid"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
