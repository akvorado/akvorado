package stack_test

import (
	"strings"
	"testing"

	"flowexporter/helpers"
	"flowexporter/reporter/stack"
)

func TestSourceFile(t *testing.T) {
	callers := stack.Callers()
	got := []string{}
	for _, caller := range callers[:len(callers)-1] {
		got = append(got, caller.SourceFile(false))
	}
	expected := []string{
		"flowexporter/reporter/stack/root_test.go",
		"testing/testing.go",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("SourceFile() (-got, +want):\n%s", diff)
	}
}

func TestFunctionName(t *testing.T) {
	callers := stack.Callers()
	got := []string{}
	for _, caller := range callers[:len(callers)-1] {
		got = append(got, caller.FunctionName())
	}
	expected := []string{
		"flowexporter/reporter/stack_test.TestFunctionName",
		"testing.tRunner",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("SourceFile() (-got, +want):\n%s", diff)
	}
}

func TestModuleName(t *testing.T) {
	got := strings.Split(stack.ModuleName, "/")
	expected := []string{"flowexporter"}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("moduleName:\n%s", diff)
	}
}
