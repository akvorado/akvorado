// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import "github.com/google/go-cmp/cmp"

// RegisterSubnetMapCmp register a subnetmap to work with cmp.Equal()/cmp.Diff()
func RegisterSubnetMapCmp[T any]() {
	RegisterCmpOption(
		cmp.Transformer("subnetmap.Transform",
			func(sm *SubnetMap[T]) map[string]T {
				return sm.ToMap()
			}))
}

func init() {
	RegisterSubnetMapCmp[uint16]()
	RegisterSubnetMapCmp[uint]()
	RegisterSubnetMapCmp[string]()
}
