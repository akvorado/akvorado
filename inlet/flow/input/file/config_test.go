// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package file

import (
	"testing"

	"akvorado/common/helpers"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(Configuration{
		Paths: []string{"/path/1", "/path/2"},
	}); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
