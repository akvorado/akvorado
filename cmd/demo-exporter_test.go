// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"testing"

	"akvorado/common/reporter"
)

func TestDemoExporterStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := DemoExporterConfiguration{}
	config.Reset()
	if err := demoExporterStart(r, config, true); err != nil {
		t.Fatalf("demoExporterStart() error:\n%+v", err)
	}
}
