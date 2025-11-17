// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"errors"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestSaveAndRestore(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	config := DefaultConfiguration()
	config.StatePersistFile = filepath.Join(t.TempDir(), "state")
	c, err := New(r, config, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}

	bf := sch.NewFlowMessage()
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "decoder", "netflow", "testdata")

	for _, pcap := range []string{"options-template.pcap", "options-data.pcap", "template.pcap"} {
		data := helpers.ReadPcapL4(t, path.Join(base, pcap))
		rawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          data,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}
		err := c.Decode(rawFlow, bf, func() {})
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}

	// Create a second component that will reuse saved templates.
	r2 := reporter.NewMock(t)
	c2, err := New(r2, config, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c2.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	got := []*schema.FlowMessage{}
	for _, pcap := range []string{"data.pcap"} {
		data := helpers.ReadPcapL4(t, path.Join(base, pcap))
		rawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          data,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}
		err := c2.Decode(rawFlow, bf, func() {
			clone := *bf
			got = append(got, &clone)
			bf.Finalize()
		})
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	if len(got) == 0 {
		t.Fatalf("Decode() returned no flows")
	}
}

func TestRestoreCorruptedFile(t *testing.T) {
	// Create a file with invalid data
	tmpDir := t.TempDir()
	corruptedFile := filepath.Join(tmpDir, "corrupted.json")
	err := os.WriteFile(corruptedFile, []byte("not valid JSON data"), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error:\n%+v", err)
	}

	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	config := DefaultConfiguration()
	config.StatePersistFile = corruptedFile
	c, err := New(r, config, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	err = c.RestoreState(corruptedFile)
	if err == nil {
		t.Error("Restore(): no error")
	}
}

func TestRestoreVersionMismatch(t *testing.T) {
	// Create a file with a different version number
	tmpDir := t.TempDir()
	versionMismatchFile := filepath.Join(tmpDir, "version_mismatch.json")

	// Write a JSON file with version 999 (incompatible version)
	incompatibleData := `{"version":999,"collection":{}}`
	err := os.WriteFile(versionMismatchFile, []byte(incompatibleData), 0644)
	if err != nil {
		t.Fatalf("WriteFile() error:\n%+v", err)
	}

	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	config := DefaultConfiguration()
	config.StatePersistFile = versionMismatchFile
	c, err := New(r, config, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	err = c.RestoreState(versionMismatchFile)
	if err == nil {
		t.Fatal("Restore(): expected error for version mismatch, got nil")
	}
	if !errors.Is(err, ErrStateVersion) {
		t.Errorf("Restore(): expected ErrVersion, got %v", err)
	}

	// Also check we have c.decoders OK
	names := []string{}
	for _, d := range c.decoders {
		names = append(names, d.Name())
	}
	slices.Sort(names)
	if diff := helpers.Diff(names, []string{"gob", "netflow", "sflow"}); diff != "" {
		t.Fatalf("RestoreState(): invalid decoders:\n%s", diff)
	}
}
