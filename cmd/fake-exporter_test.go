// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"testing"

	"akvorado/common/reporter"
)

func TestFakeExporterStart(t *testing.T) {
	r := reporter.NewMock(t)
	config := FakeExporterConfiguration{}
	config.Reset()
	if err := fakeExporterStart(r, config, true); err != nil {
		t.Fatalf("fakeExporterStart() error:\n%+v", err)
	}
}
