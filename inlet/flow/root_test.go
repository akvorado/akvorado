// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/input/file"
)

func TestFlow(t *testing.T) {
	var nominalRate int
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "decoder", "netflow", "testdata")
	outDir := t.TempDir()
	outFiles := []string{}
	for idx, f := range []string{
		"options-template.pcap",
		"options-data.pcap",
		"template.pcap",
		"data.pcap", "data.pcap", "data.pcap", "data.pcap",
		"data.pcap", "data.pcap", "data.pcap", "data.pcap",
		"data.pcap", "data.pcap", "data.pcap", "data.pcap",
		"data.pcap", "data.pcap", "data.pcap", "data.pcap",
	} {
		outFile := path.Join(outDir, fmt.Sprintf("data-%d", idx))
		err := os.WriteFile(outFile, helpers.ReadPcapL4(t, path.Join(base, f)), 0o666)
		if err != nil {
			t.Fatalf("WriteFile(%q) error:\n%+v", outFile, err)
		}
		outFiles = append(outFiles, outFile)
	}

	inputs := []InputConfiguration{
		{
			Decoder: "netflow",
			Config: &file.Configuration{
				Paths: outFiles,
			},
		},
	}

	for retry := 2; retry >= 0; retry-- {
		// Without rate limiting
		{
			r := reporter.NewMock(t)
			config := DefaultConfiguration()
			config.Inputs = inputs
			c := NewMock(t, r, config)

			// Receive flows
			now := time.Now()
			for range 1000 {
				select {
				case <-c.Flows():
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("no flow received")
				}
			}
			elapsed := time.Now().Sub(now)
			t.Logf("Elapsed time for 1000 messages is %s", elapsed)
			nominalRate = int(1000 * (time.Second / elapsed))
		}

		// With rate limiting
		if runtime.GOOS == "Linux" {
			r := reporter.NewMock(t)
			config := DefaultConfiguration()
			config.RateLimit = 1000
			config.Inputs = inputs
			c := NewMock(t, r, config)

			// Receive flows
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
				t.Logf("Nominal rate was %d/second", nominalRate)
				expectedRate := uint64(30000 / 1000 * nominalRate)
				if flow.SamplingRate > uint32(1000*expectedRate/100) || flow.SamplingRate < uint32(70*expectedRate/100) {
					if retry > 0 {
						continue
					}
					t.Fatalf("Sampling rate is %d, expected %d", flow.SamplingRate, expectedRate)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("no flow received")
			}
			break
		}
	}
}
