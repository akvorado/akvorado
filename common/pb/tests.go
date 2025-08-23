// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package pb

import (
	"akvorado/common/helpers"

	"github.com/google/go-cmp/cmp/cmpopts"
)

func init() {
	helpers.RegisterCmpOption(cmpopts.IgnoreUnexported(RawFlow{}))
}
