// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestDeadCodeElimination(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("unsupported OS: %s", runtime.GOOS)
	}
	const (
		code = `
package main

import "akvorado/cmd"

func main() {
  cmd.RootCmd.Execute()
}`
		dirname    = "test_deadcode"
		progname   = "test_deadcode_elimination"
		symbolname = "akvorado/cmd.(*DemoExporterConfiguration).UnusedButExportedFunction"
	)
	// Get flags from Makefile
	makefile, err := os.ReadFile(filepath.Join("..", "Makefile"))
	if err != nil {
		t.Fatalf("ReadFile() error:\n%+v", err)
	}
	// Look for target "all:", then "-tags ..."
	lines := strings.Split(string(makefile), "\n")
	inAllTarget := false
	tagsRe := regexp.MustCompile(`^\s+-tags\s+(\S+)\s+\\?\s*$`)
	var tags string
	for _, line := range lines {
		if strings.HasPrefix(line, "all:") {
			inAllTarget = true
			continue
		}
		if inAllTarget {
			// Check if we've left the target (new target or non-indented line)
			if len(line) > 0 && line[0] != '\t' && line[0] != ' ' {
				inAllTarget = false
				break
			}
			if matches := tagsRe.FindStringSubmatch(line); matches != nil {
				tags = matches[1]
				break
			}
		}
	}
	if tags == "" {
		t.Fatalf("Could not find -tags in Makefile 'all' target")
	}
	t.Logf("Compilation tags: %s", tags)

	// Build test program
	_ = os.Mkdir(dirname, 0770)
	defer os.RemoveAll(dirname)
	filename := filepath.Join(dirname, fmt.Sprintf("%s.go", progname))
	if err := os.WriteFile(filename, []byte(code), 0600); err != nil {
		t.Fatalf("WriteFile() error:\n%+v", err)
	}
	if buf, err := exec.Command("go", "build", "-tags", tags, filename).CombinedOutput(); err != nil {
		t.Fatalf("go build error:\n%s", buf)
	}
	defer os.Remove(progname)

	// Check if symbol is present
	f, err := elf.Open(progname)
	if err != nil {
		t.Fatalf("elf.Open() error:\n%+v", err)
	}
	defer f.Close()

	symbols, err := f.Symbols()
	if err != nil {
		t.Fatalf("Symbols() error:\n%+v", err)
	}

	for _, sym := range symbols {
		if sym.Name == symbolname {
			t.Errorf("Expected %s to be eliminated, but it was found in binary", symbolname)
			t.Log("Use `make all BUILD_ARGS=\"-ldflags=-dumpdep\" |& go run github.com/aarzilli/whydeadcode@latest`")
			break
		}
	}
}
