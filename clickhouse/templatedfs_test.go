package clickhouse

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"testing"

	"akvorado/helpers"
)

//go:embed testdata
var testFS embed.FS

func TestTemplatedFS(t *testing.T) {
	templated := &templatedFS{map[string]string{"Name": "world"}, testFS}
	entries, err := fs.ReadDir(templated, "testdata")
	if err != nil {
		t.Fatalf("ReadDir() error:\n%+v", err)
	}
	expectedEntries := []string{
		"regular-file.txt",
		"templated-file.txt",
		"templated-file-with-error.txt",
	}
	gotEntries := []string{}
	for _, entry := range entries {
		gotEntries = append(gotEntries, entry.Name())
	}
	sort.Strings(expectedEntries)
	sort.Strings(gotEntries)
	if diff := helpers.Diff(gotEntries, expectedEntries); diff != "" {
		t.Errorf("ReadDir() (-got, +want):\n%s", diff)
	}

	cases := []struct {
		File    string
		Content string
		Err     bool
	}{
		{
			File:    "regular-file.txt",
			Content: "hello\n",
		}, {
			File:    "templated-file.txt",
			Content: "Hello world!\n",
		}, {
			File: "templated-file-with-error.txt",
			Err:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			got, err := fs.ReadFile(templated, fmt.Sprintf("testdata/%s", tc.File))
			if err != nil && !tc.Err {
				t.Fatalf("ReadFile() error:\n%+v", err)
			}
			if err == nil && tc.Err {
				t.Fatalf("ReadFile() did not return an error")
			}
			if diff := helpers.Diff(string(got), tc.Content); diff != "" {
				t.Fatalf("ReadFile() (-got, +want):\n%s", diff)
			}
		})
	}

}
