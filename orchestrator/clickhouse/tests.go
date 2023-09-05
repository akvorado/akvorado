// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package clickhouse

import (
	"fmt"
	"reflect"

	"akvorado/common/helpers"

	"github.com/itchyny/gojq"
)

// MustParseTransformQuery parses a transform query or panic.
func MustParseTransformQuery(src string) TransformQuery {
	q, err := gojq.Parse(src)
	if err != nil {
		panic(err)
	}
	return TransformQuery{q}
}

func init() {
	helpers.AddPrettyFormatter(reflect.TypeOf(helpers.SubnetMap[NetworkAttributes]{}), fmt.Sprint)
}
