// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/inlet/metadata/provider"
)

func TestStateUpdate(t *testing.T) {
	var state exporterState
	var model Model
	for _, model = range DefaultModels() {
		if model.Name == "Nokia SR Linux" {
			break
		}
	}
	if model.Name != "Nokia SR Linux" {
		t.Fatal("Nokia SR Linux model not found")
	}

	// No events
	state.update([]event{}, model)
	expected := exporterState{
		Name:       "",
		Interfaces: map[uint]provider.Interface{},
	}
	if diff := helpers.Diff(state, expected); diff != "" {
		t.Fatalf("udpate() (-got, +want):\n%s", diff)
	}

	// Some events
	state.update([]event{
		{"/interface/description", "name=ethernet-1/1", "1st interface"},
		{"/interface/description", "name=ethernet-1/2", "2nd interface"},
		{"/interface/description", "name=ethernet-1/3", "3rd interface"},
		{"/interface/description", "name=lag1", "lag interface"},
		{"/interface/ethernet/port-speed", "name=ethernet-1/1", "100G"},
		{"/interface/ethernet/port-speed", "name=ethernet-1/2", "100G"},
		{"/interface/ethernet/port-speed", "name=ethernet-1/3", "10G"},
		{"/interface/ethernet/port-speed", "name=ethernet-1/4", "25G"},
		{"/interface/ethernet/port-speed", "name=mgmt0", "1G"},
		{"/interface/lag/lag-speed", "name=lag1", "100000000"},
		{"/interface/subinterface/description", "name=ethernet-1/4,index=1", "4th interface"},
		{"/interface/subinterface/name", "name=ethernet-1/4,index=1", "ethernet-1/4.1"},
		{"/interface/subinterface/name", "name=mgmt0,index=0", "mgmt0.0"},
		{"/interface/ifindex", "name=ethernet-1/1", "100"},
		{"/interface/ifindex", "name=ethernet-1/2", "101"},
		{"/interface/ifindex", "name=ethernet-1/3", "102"},
		{"/interface/ifindex", "name=ethernet-1/4", "103"},
		{"/interface/subinterface/ifindex", "name=ethernet-1/4,index=1", "105"},
		{"/interface/ifindex", "name=lag1", "106"},
		{"/system/name/host-name", "", "srlinux"},
	}, model)
	expected = exporterState{
		Name: "srlinux",
		Interfaces: map[uint]provider.Interface{
			100: {
				Name:        "ethernet-1/1",
				Description: "1st interface",
				Speed:       100_000,
			},
			101: {
				Name:        "ethernet-1/2",
				Description: "2nd interface",
				Speed:       100_000,
			},
			102: {
				Name:        "ethernet-1/3",
				Description: "3rd interface",
				Speed:       10_000,
			},
			103: {
				Name:        "ethernet-1/4",
				Description: "",
				Speed:       25_000,
			},
			105: {
				Name:        "ethernet-1/4.1",
				Description: "4th interface",
				Speed:       25_000,
			},
			106: {
				Name:        "lag1",
				Description: "lag interface",
				Speed:       100,
			},
		},
	}
	if diff := helpers.Diff(state, expected); diff != "" {
		t.Fatalf("udpate() (-got, +want):\n%s", diff)
	}
}
