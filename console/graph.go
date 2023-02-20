// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"time"

	"akvorado/common/schema"
	"akvorado/console/query"
)

// graphCommonHandlerInput is for bits common to graphLineHandlerInput and
// graphSankeyHandlerInput.
type graphCommonHandlerInput struct {
	schema     *schema.Component
	Start      time.Time      `json:"start" binding:"required"`
	End        time.Time      `json:"end" binding:"required,gtfield=Start"`
	Dimensions []query.Column `json:"dimensions"`            // group by ...
	Limit      int            `json:"limit" binding:"min=1"` // limit product of dimensions
	Filter     query.Filter   `json:"filter"`                // where ...
	Units      string         `json:"units" binding:"required,oneof=pps l3bps l2bps inl2% outl2%"`
}
