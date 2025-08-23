// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package filter

import (
	"akvorado/common/helpers"

	"github.com/google/go-cmp/cmp/cmpopts"
)

func init() {
	helpers.RegisterCmpOption(cmpopts.IgnoreFields(Meta{}, "Schema"))
}
