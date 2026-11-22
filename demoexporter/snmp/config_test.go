// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"testing"

	"akvorado/common/helpers"
)

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	config.Name = "demo"
	config.Interfaces = map[int]InterfaceConfiguration{
		1: {Description: "Transit: Cogent", Speed: 10000},
		2: {Description: "Core", Speed: 100000},
		3: {Description: "Core", Speed: 100000},
		6: {Description: "PNI: Netflix", Speed: 10000, Name: "Hu0/0/0/6"},
	}
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
