// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"strings"
	"time"

	"akvorado/common/schema"
	"akvorado/console/query"
)

// graphCommonHandlerInput is for bits common to graphLineHandlerInput and
// graphSankeyHandlerInput.
type graphCommonHandlerInput struct {
	schema         *schema.Component
	Start          time.Time      `json:"start" binding:"required"`
	End            time.Time      `json:"end" binding:"required,gtfield=Start"`
	Dimensions     []query.Column `json:"dimensions"`            // group by ...
	Limit          int            `json:"limit" binding:"min=1"` // limit product of dimensions
	LimitType      string         `json:"limitType" validate:"oneof=avg max last"`
	Filter         query.Filter   `json:"filter"`                              // where ...
	TruncateAddrV4 int            `json:"truncate-v4" binding:"min=0,max=32"`  // 0 or 32 = no truncation
	TruncateAddrV6 int            `json:"truncate-v6" binding:"min=0,max=128"` // 0 or 128 = no truncation
	Units          string         `json:"units" binding:"required,oneof=pps l3bps l2bps inl2% outl2%"`
}

// sourceSelect builds a SELECT query to use as a source for data. Notably, it
// will do IP truncation.
func (input graphCommonHandlerInput) sourceSelect() string {
	if input.TruncateAddrV4 == 0 {
		input.TruncateAddrV4 = 32
	}
	if input.TruncateAddrV6 == 0 {
		input.TruncateAddrV6 = 128
	}
	truncated := []string{}
	for _, qc := range input.Dimensions {
		if column, _ := input.schema.LookupColumnByKey(qc.Key()); column.ConsoleTruncateIP {
			if input.TruncateAddrV4 == 32 && input.TruncateAddrV6 == 128 {
				continue
			}
			if input.TruncateAddrV6 == input.TruncateAddrV4+96 {
				truncated = append(truncated,
					fmt.Sprintf("tupleElement(IPv6CIDRToRange(%s, %d), 1) AS %s",
						qc.String(), input.TruncateAddrV6, qc.String()))
			} else {
				truncated = append(truncated,
					fmt.Sprintf("tupleElement(IPv6CIDRToRange(%s, if(tupleElement(IPv6CIDRToRange(%s, 96), 1) = toIPv6('::ffff:0.0.0.0'), %d, %d)), 1) AS %s",
						qc.String(), qc.String(),
						input.TruncateAddrV4+96, input.TruncateAddrV6,
						qc.String()))
			}
		}
	}
	if len(truncated) == 0 {
		return "SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1"
	}
	return fmt.Sprintf("SELECT * REPLACE (%s) FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1", strings.Join(truncated, ", "))
}
