// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

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
	var nominalRate int
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "decoder", "netflow", "testdata")
	inputs := []InputConfiguration{
		{
			Decoder: "netflow",
			Config: &file.Configuration{
				Paths: []string{
					path.Join(base, "options-template-257.data"),
					path.Join(base, "options-data-257.data"),
					path.Join(base, "template-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
					path.Join(base, "data-260.data"),
				},
			},
		},
	}
	t.Run("without rate limit", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		config.Inputs = inputs
		c := NewMock(t, r, config)

		// Receive flows
		now := time.Now()
		for i := 0; i < 1000; i++ {
			select {
			case <-c.Flows():
			case <-time.After(30 * time.Millisecond):
				t.Fatalf("no flow received")
			}
		}
		elapsed := time.Now().Sub(now)
		t.Logf("Elapsed time for 1000 messages is %s", elapsed)
		nominalRate = int(1000 * (time.Second / elapsed))
	})

	t.Run("with rate limit", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		config.RateLimit = 1000
		config.Inputs = inputs
		c := NewMock(t, r, config)

		// Receive flows. It's a bit difficult to estimate the rate as
		// they will come as fast as possible. We'll get an estimate
		// during the first second.
		twoSeconds := time.After(2 * time.Second)
		count := 0
	outer1:
		for {
			select {
			case <-c.Flows():
				count++
			case <-twoSeconds:
				break outer1
			}
		}
		t.Logf("During the first two seconds, got %d flows", count)
		t.Logf("Nominal rate was %d/second", nominalRate)

		if count > 2200 || count < 2000 {
			t.Fatalf("Got %d flows instead of 2100 (burst included)", count)
		}

		if nominalRate == 0 {
			return
		}
		select {
		case flow := <-c.Flows():
			// This is hard to estimate the number of
			// flows we should have got. We use the
			// nominal rate but it was done with rate
			// limiting disabled (so less code).
			// Therefore, we are super conservative on the
			// upper limit of the sampling rate. However,
			// the lower limit should be OK.
			expectedRate := uint64(30000 / 1000 * nominalRate)
			if flow.SamplingRate > 1000*expectedRate/100 || flow.SamplingRate < 70*expectedRate/100 {
				t.Fatalf("Sampling rate is %d, expected %d", flow.SamplingRate, expectedRate)
			}
		case <-time.After(30 * time.Millisecond):
			t.Fatalf("no flow received")
		}
	})
}
