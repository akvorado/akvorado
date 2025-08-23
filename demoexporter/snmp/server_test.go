// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"context"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/gosnmp/gosnmp"
)

func TestSNMPServer(t *testing.T) {
	config := Configuration{
		Name: "demo",
		Interfaces: map[int]string{
			1: "transit: cogent",
			2: "pni: netflix",
		},
		Listen: "127.0.0.1:0",
	}
	r := reporter.NewMock(t)
	c := NewMock(t, r, config)

	g := &gosnmp.GoSNMP{
		Target:                  "127.0.0.1",
		Port:                    uint16(c.snmpPort),
		Community:               "public",
		Version:                 gosnmp.Version2c,
		Context:                 context.Background(),
		Retries:                 3,
		Timeout:                 time.Second,
		UseUnconnectedUDPSocket: true,
	}
	if err := g.Connect(); err != nil {
		t.Fatalf("Connect() error:\n%+v", err)
	}
	got := []gosnmp.SnmpPDU{}
	if err := g.Walk("1.3.6.1.2.1", func(data gosnmp.SnmpPDU) error {
		got = append(got, data)
		return nil
	}); err != nil {
		t.Fatalf("Walk() error:\n%+v", err)
	}

	expected := []gosnmp.SnmpPDU{
		{
			Name:  ".1.3.6.1.2.1.1.5.0",
			Value: []byte("demo"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.2.2.1.2.1",
			Value: []byte("Gi0/0/0/1"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.2.2.1.2.2",
			Value: []byte("Gi0/0/0/2"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.1.1",
			Value: []byte("Gi0/0/0/1"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.1.2",
			Value: []byte("Gi0/0/0/2"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.15.1",
			Value: uint(10000),
			Type:  gosnmp.Gauge32,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.15.2",
			Value: uint(10000),
			Type:  gosnmp.Gauge32,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.18.1",
			Value: []byte("transit: cogent"),
			Type:  gosnmp.OctetString,
		}, {
			Name:  ".1.3.6.1.2.1.31.1.1.1.18.2",
			Value: []byte("pni: netflix"),
			Type:  gosnmp.OctetString,
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Walk() (-got, +want):\n%s", diff)
	}

	gotMetrics := r.GetMetrics("akvorado_demoexporter_")
	expectedMetrics := map[string]string{
		`snmp_requests_total{oid="1.3.6.1.2.1.1.5.0"}`:         "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.2.2.1.2.1"}`:     "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.2.2.1.2.2"}`:     "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.1.1"}`:  "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.1.2"}`:  "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.15.1"}`: "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.15.2"}`: "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.18.1"}`: "1",
		`snmp_requests_total{oid="1.3.6.1.2.1.31.1.1.1.18.2"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
