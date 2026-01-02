// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"testing"

	"akvorado/common/constants"
	"akvorado/common/helpers"
)

func TestFlowSettings(t *testing.T) {
	expected := map[uint16]*flowFamilySettings{
		constants.ETypeIPv4: {
			MaxFlowsPerPacket: 28,
			FlowLength:        50,
			TemplateID:        260,
			Template:          flowSettings[constants.ETypeIPv4].Template,
		},
		constants.ETypeIPv6: {
			MaxFlowsPerPacket: 18,
			FlowLength:        74,
			TemplateID:        261,
			Template:          flowSettings[constants.ETypeIPv6].Template,
		},
	}
	if diff := helpers.Diff(flowSettings, expected); diff != "" {
		t.Fatalf("flowSettings (-got, +want):\n%s", diff)
	}
}
