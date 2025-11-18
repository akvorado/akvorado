// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package stack_test

import (
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter/stack"
)

func TestSourceFile(t *testing.T) {
	callers := stack.Callers()
	got := []string{}
	for _, caller := range callers[:len(callers)-1] {
		// Extract source file without line number
		sourceFile := caller.Info().SourceFile()
		if idx := strings.LastIndex(sourceFile, ":"); idx != -1 {
			sourceFile = sourceFile[:idx]
		}
		got = append(got, sourceFile)
	}
	expected := []string{
		"akvorado/common/reporter/stack/root_test.go",
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
		got = append(got, caller.Info().FunctionName())
	}
	expected := []string{
		"akvorado/common/reporter/stack_test.TestFunctionName",
		"testing.tRunner",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("FunctionName() (-got, +want):\n%s", diff)
	}
}

func TestModuleName(t *testing.T) {
	got := strings.Split(stack.ModuleName, "/")
	expected := []string{"akvorado"}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("moduleName:\n%s", diff)
	}
}

func TestInfo(t *testing.T) {
	callers := stack.Callers()
	if len(callers) == 0 {
		t.Fatal("Callers() empty")
	}
	caller := callers[0]
	info := caller.Info()

	// Verify basic properties
	functionName := info.FunctionName()
	if !strings.HasSuffix(functionName, ".TestInfo") {
		t.Errorf("FunctionName() should end with .TestInfo, got %q", functionName)
	}

	fileName := info.FileName()
	if !strings.HasSuffix(fileName, "/root_test.go") {
		t.Errorf("FileName() should end with /root_test.go, got %q", fileName)
	}

	sourceFile := info.SourceFile()
	if !strings.HasSuffix(sourceFile, "akvorado/common/reporter/stack/root_test.go:58") {
		t.Errorf("SourceFile() should have complete path, got %q", sourceFile)
	}
}

func BenchmarkCallInfo(b *testing.B) {
	callers := stack.Callers()
	if len(callers) == 0 {
		b.Fatal("no callers")
	}
	caller := callers[0]

	b.Run("Info() all fields", func(b *testing.B) {
		for b.Loop() {
			info := caller.Info()
			_ = info.FunctionName()
			_ = info.FileName()
			_ = info.SourceFile()
		}
	})

	b.Run("Info() function name", func(b *testing.B) {
		for b.Loop() {
			info := caller.Info()
			_ = info.FunctionName()
		}
	})

	b.Run("Info() function and filename", func(b *testing.B) {
		// Simulates frames that match but we skip SourceFile computation
		for b.Loop() {
			info := caller.Info()
			_ = info.FunctionName()
			_ = info.FileName()
		}
	})

	b.Run("Info() separate", func(b *testing.B) {
		for b.Loop() {
			_ = caller.Info().FunctionName()
			_ = caller.Info().FileName()
			_ = caller.Info().SourceFile()
		}
	})
}
