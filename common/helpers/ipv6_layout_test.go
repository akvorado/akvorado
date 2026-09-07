// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net/netip"
	"reflect"
	"testing"
	"unsafe"
)

type fieldLayout struct {
	Offset uintptr
	Size   uintptr
	Align  int
}

func layoutOf(t reflect.Type) []fieldLayout {
	layout := make([]fieldLayout, t.NumField())
	for i := range layout {
		field := t.Field(i)
		layout[i] = fieldLayout{field.Offset, field.Type.Size(), field.Type.Align()}
	}
	return layout
}

// TestNetIPAddrStructure checks the layout of netip.Addr matches what we expect.
func TestNetIPAddrStructure(t *testing.T) {
	type field struct {
		Name   string
		Type   string
		Offset uintptr
		Size   uintptr
	}
	addrType := reflect.TypeFor[netip.Addr]()
	got := make([]field, addrType.NumField())
	for i := range got {
		f := addrType.Field(i)
		got[i] = field{f.Name, f.Type.String(), f.Offset, f.Type.Size()}
	}
	expected := []field{
		{"addr", "netip.uint128", 0, 16},
		{"z", "unique.Handle[net/netip.addrDetail]", 16, 8},
	}
	if diff := Diff(got, expected); diff != "" {
		t.Fatalf("netip.Addr fields (-got, +want):\n%s", diff)
	}
	if diff := Diff(addrType.Size(), uintptr(24)); diff != "" {
		t.Errorf("netip.Addr size (-got, +want):\n%s", diff)
	}

	// addr is an uint128 made of two uint64.
	for f := range addrType.Field(0).Type.Fields() {
		if diff := Diff(f.Type.Kind(), reflect.Uint64); diff != "" {
			t.Errorf("netip.uint128 field %q kind (-got, +want):\n%s", f.Name, diff)
		}
	}
	// z is a pointer
	zType := addrType.Field(1).Type
	if diff := Diff(zType.NumField(), 1); diff != "" {
		t.Fatalf("unique.Handle field count (-got, +want):\n%s", diff)
	}
	if diff := Diff(zType.Field(0).Type.Kind(), reflect.Pointer); diff != "" {
		t.Errorf("unique.Handle field kind (-got, +want):\n%s", diff)
	}
}

// TestAddrProxy checks addrProxy matches netip.Addr structure
func TestAddrProxy(t *testing.T) {
	addrType := reflect.TypeFor[netip.Addr]()
	proxyType := reflect.TypeFor[addrProxy]()

	if diff := Diff(proxyType.Size(), addrType.Size()); diff != "" {
		t.Errorf("addrProxy size (-got, +want):\n%s", diff)
	}
	if diff := Diff(layoutOf(proxyType), layoutOf(addrType)); diff != "" {
		t.Errorf("addrProxy layout (-got, +want):\n%s", diff)
	}
	if diff := Diff(proxyType.Field(1).Type.Kind(), reflect.UnsafePointer); diff != "" {
		t.Errorf("addrProxy z kind (-got, +want):\n%s", diff)
	}

	// z6noz must be the handle netip gives to an IPv6 address without a zone.
	ipv6 := netip.MustParseAddr("2001:db8::1")
	if diff := Diff((*addrProxy)(unsafe.Pointer(&ipv6)).z, z6noz); diff != "" {
		t.Errorf("z6noz (-got, +want):\n%s", diff)
	}
}
