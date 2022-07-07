// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"testing"

	"akvorado/common/helpers"
)

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	config.Name = "fake"
	config.Interfaces = map[int]string{
		1: "Transit: Cogent",
		2: "Core",
		3: "Core",
		6: "PNI: Netflix",
	}
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
