package file

import (
	"path"
	"testing"
	"time"

	"akvorado/daemon"
	"akvorado/helpers"
	"akvorado/reporter"
)

func TestFileInput(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.Paths = []string{path.Join("testdata", "file1.txt"), path.Join("testdata", "file2.txt")}
	in, err := configuration.New(r, daemon.NewMock(t))
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	ch, err := in.Start()
	if err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := in.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	// Get it back
	expected := []string{"hello world!\n", "bye bye\n", "hello world!\n"}
	got := []string{}
out:
	for i := 0; i < len(expected); i++ {
		select {
		case got1 := <-ch:
			got = append(got, string(got1.Payload))
		case <-time.After(50 * time.Millisecond):
			break out
		}
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Input data (-got, +want):\n%s", diff)
	}
}
