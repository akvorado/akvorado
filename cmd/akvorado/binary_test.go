// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func TestDeadCodeElimination(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("unsupported OS: %s", runtime.GOOS)
	}
	if testing.Short() {
		t.Skip("skip compilation of binaries in short mode")
	}
	const symbolName = "main.(*InletConfiguration).UnusedButExportedFunction"

	// Get tags from Makefile
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("ReadFile() error:\n%+v", err)
	}
	baseTags := regexp.MustCompile(`(?m)^BUILD_TAGS\s*=\s*(\S+)\s*$`).FindSubmatch(makefile)
	if baseTags == nil {
		t.Fatal("Could not find BUILD_TAGS in Makefile")
	}
	inletTags := regexp.MustCompile(`(?m)^bin/.*-inlet:\s*BUILD_TAGS\s*:=\s*\$\(BUILD_TAGS\),(\S+)\s*$`).FindSubmatch(makefile)
	if inletTags == nil {
		t.Fatal("Could not find BUILD_TAGS for akvorado-inlet in Makefile")
	}

	cases := []struct {
		Name string
		Tags string
		Skip bool
	}{
		{
			Name: "akvorado-inlet",
			Tags: fmt.Sprintf("%s,%s", baseTags[1], inletTags[1]),
		}, {
			Name: "akvorado",
			Tags: string(baseTags[1]),
			Skip: true, // not working yet
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Skip {
				t.Skip("not ready for DCE")
			}
			t.Logf("Compilation tags: %s", tc.Tags)
			progname := filepath.Join(t.TempDir(), tc.Name)
			if buf, err := exec.Command("go", "build", "-tags", tc.Tags, "-o", progname, ".").CombinedOutput(); err != nil {
				t.Fatalf("go build error:\n%s", buf)
			}

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
				if sym.Name == symbolName {
					t.Errorf("Expected %s to be eliminated, but it was found in binary", symbolName)
					t.Logf("Use `make bin/%s BUILD_ARGS=\"-ldflags=-dumpdep\" |& go run github.com/aarzilli/whydeadcode@latest`", tc.Name)
					return
				}
			}
		})
	}
}
