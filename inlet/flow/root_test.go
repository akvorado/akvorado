package flow

import (
	"path"
	"runtime"
	"testing"
	"time"

	"akvorado/common/reporter"
	"akvorado/inlet/flow/input/file"
)

func TestFlow(t *testing.T) {
	r := reporter.NewMock(t)
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "decoder", "netflow", "testdata")
	config := DefaultConfiguration()
	config.Inputs = []InputConfiguration{
		{
			Decoder: "netflow",
			Config: &file.Configuration{
				Paths: []string{
					path.Join(base, "options-template-257.data"),
					path.Join(base, "options-data-257.data"),
					path.Join(base, "template-260.data"),
					path.Join(base, "data-260.data"),
				},
			},
		},
	}
	c := NewMock(t, r, config)
	defer func() {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	// Receive flows
	received := []*Message{}
	for i := 0; i < 10; i++ {
		select {
		case flow := <-c.Flows():
			received = append(received, flow)
		case <-time.After(30 * time.Millisecond):
			t.Fatalf("no flow received")
		}
	}
}
