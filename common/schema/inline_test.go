// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux && !race

package schema

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strings"
	"testing"
)

// inlineDirective marks a function which must be inlined at each of its call
// sites.
const inlineDirective = "//akvorado:inline"

// packagePath is the prefix the linker uses for the symbols of this package.
const packagePath = "akvorado/common/schema"

//akvorado:inline
func notInlined(t *testing.T) {
	t.Log("I am too big to be inlined (1)")
	t.Log("I am too big to be inlined (2)")
	t.Log("I am too big to be inlined (3)")
}

//akvorado:inline
func inlined(t *testing.T) {
	t.Log("I should be inlined")
}

// TestInline checks the functions marked with the inline directive are inlined
// at each of their call sites. When a function is inlined everywhere, nothing
// references its body anymore and the linker removes it from the executable:
// looking for it in the test executable is therefore enough to tell.
func TestInline(t *testing.T) {
	if testing.CoverMode() != "" {
		// Coverage adds counters inside each function and makes them
		// more expensive to inline.
		t.Skip("inlining decisions are different with coverage")
	}

	// The control functions have to be called to be part of the executable.
	notInlined(t)
	inlined(t)

	linked := linkedFunctions(t)
	marked := markedFunctions(t)
	if len(marked) == 0 {
		t.Fatalf("no function marked with %q", inlineDirective)
	}
	control := fmt.Sprintf("%s.notInlined", packagePath)
	for _, name := range marked {
		if name == control {
			// Reversed check: without it, a function we cannot look
			// up would be reported as inlined, for example if the
			// names do not have the shape we expect.
			if !linked[name] {
				t.Errorf("%s() not found in the test executable", name)
			}
			continue
		}
		if linked[name] {
			t.Errorf("%s() is not inlined", name)
		}
	}
}

// markedFunctions parses the source files of the current package and returns
// the names the linker gives to the functions carrying the inline directive.
func markedFunctions(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir() error:\n%+v", err)
	}
	fset := token.NewFileSet()
	marked := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error:\n%+v", name, err)
		}
		if file.Name.Name != path.Base(packagePath) {
			// The external test package uses another prefix
			continue
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || !hasInlineDirective(function) {
				continue
			}
			symbol, ok := functionSymbol(function)
			if !ok {
				t.Fatalf("cannot compute symbol name for %s() in %s", function.Name.Name, name)
			}
			marked = append(marked, symbol)
		}
	}
	return marked
}

// hasInlineDirective tells if the provided function carries the inline
// directive.
func hasInlineDirective(function *ast.FuncDecl) bool {
	if function.Doc == nil {
		return false
	}
	for _, comment := range function.Doc.List {
		if comment.Text == inlineDirective {
			return true
		}
	}
	return false
}

// functionSymbol returns the name the linker gives to the provided function.
// The second return value is false when the name cannot be computed, notably
// for a generic receiver.
func functionSymbol(function *ast.FuncDecl) (string, bool) {
	if function.Recv == nil {
		return fmt.Sprintf("%s.%s", packagePath, function.Name.Name), true
	}
	if len(function.Recv.List) != 1 {
		return "", false
	}
	switch receiver := function.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return fmt.Sprintf("%s.%s.%s", packagePath, receiver.Name, function.Name.Name), true
	case *ast.StarExpr:
		if ident, ok := receiver.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.(*%s).%s", packagePath, ident.Name, function.Name.Name), true
		}
	}
	return "", false
}

// linkedFunctions returns the names of the functions kept in the test
// executable.
func linkedFunctions(t *testing.T) map[string]bool {
	t.Helper()
	executable, err := elf.Open("/proc/self/exe")
	if err != nil {
		t.Fatalf("elf.Open() error:\n%+v", err)
	}
	defer executable.Close()

	// A test executable is linked without its symbol table, but the table
	// used to build tracebacks is always there and only describes the
	// functions present in the text section.
	text := executable.Section(".text")
	pclntab := executable.Section(".gopclntab")
	if text == nil || pclntab == nil {
		t.Fatal("cannot find the traceback table in the test executable")
	}
	data, err := pclntab.Data()
	if err != nil {
		t.Fatalf("Data() error:\n%+v", err)
	}
	table, err := gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		t.Fatalf("gosym.NewTable() error:\n%+v", err)
	}
	linked := make(map[string]bool, len(table.Funcs))
	for _, function := range table.Funcs {
		linked[function.Name] = true
	}
	return linked
}
