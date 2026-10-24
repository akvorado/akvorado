// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/filter"
	"akvorado/console/query"
)

// graphCommonHandlerInput is for bits common to graphLineHandlerInput and
// graphSankeyHandlerInput.
type graphCommonHandlerInput struct {
	schema         *schema.Component
	database       string
	Start          time.Time      `json:"start" validate:"required"`
	End            time.Time      `json:"end" validate:"required,gtfield=Start"`
	Dimensions     []query.Column `json:"dimensions"`             // group by ...
	Limit          int            `json:"limit" validate:"min=1"` // limit product of dimensions
	LimitType      string         `json:"limitType" validate:"omitempty,oneof=avg max last"`
	Filter         query.Filter   `json:"filter"`                               // where ...
	TruncateAddrV4 int            `json:"truncate-v4" validate:"min=0,max=32"`  // 0 or 32 = no truncation
	TruncateAddrV6 int            `json:"truncate-v6" validate:"min=0,max=128"` // 0 or 128 = no truncation
	Units          string         `json:"units" validate:"required,oneof=fps pps l3bps l2bps inl2% outl2%"`
}

// reverseUnits returns the unit name for the opposite traffic direction. Most
// units are direction-agnostic; only the percentage-of-interface units swap.
func reverseUnits(units string) string {
	switch units {
	case "inl2%":
		return "outl2%"
	case "outl2%":
		return "inl2%"
	}
	return units
}

// truncateIP cuts an address down to the network of the requested prefix
// length.
func truncateIP(addr, bits sb.Expr) sb.Expr {
	return sb.Function("tupleElement",
		sb.Function("IPv6CIDRToRange", addr, bits),
		sb.Uint(1))
}

// sourceSelect builds a SELECT query to use as a source for data. Notably, it
// will do IP truncation.
func (input graphCommonHandlerInput) sourceSelect(table string) *sb.Query {
	if input.TruncateAddrV4 == 0 {
		input.TruncateAddrV4 = 32
	}
	if input.TruncateAddrV6 == 0 {
		input.TruncateAddrV6 = 128
	}
	truncated := []sb.Expr{}
	for _, qc := range input.Dimensions {
		if column, _ := input.schema.LookupColumnByKey(qc.Key()); column.ConsoleTruncateIP {
			if input.TruncateAddrV4 == 32 && input.TruncateAddrV6 == 128 {
				continue
			}
			addr := sb.Column(qc.String())
			bits := sb.Uint(uint64(input.TruncateAddrV6))
			if input.TruncateAddrV6 != input.TruncateAddrV4+96 {
				// Addresses are all stored as IPv6 ones, so the prefix length
				// to use depends on the family of each address.
				bits = sb.Function("if",
					sb.Op(truncateIP(addr, sb.Uint(96)), "=",
						sb.Function("toIPv6", sb.String("0.0.0.0"))),
					sb.Uint(uint64(input.TruncateAddrV4+96)),
					sb.Uint(uint64(input.TruncateAddrV6)))
			}
			truncated = append(truncated,
				sb.Alias(truncateIP(addr, bits), qc.String()))
		}
	}
	source := sb.Select()
	if len(truncated) == 0 {
		source.Item(sb.Star())
	} else {
		source.Item(sb.Star(), sb.Replace(truncated...))
	}
	return source.From(sb.Table(table)).
		Setting("asterisk_include_alias_columns", sb.Uint(1))
}

// graphReverseHandlerInput describes the input of the /graph/reverse endpoint.
type graphReverseHandlerInput struct {
	Dimensions []query.Column `json:"dimensions"`
	Filter     string         `json:"filter"`
}

// graphReverseHandlerOutput describes the output of the /graph/reverse endpoint.
type graphReverseHandlerOutput struct {
	Dimensions []query.Column `json:"dimensions"`
	Filter     string         `json:"filter"`
}

// graphReverseHandlerFunc returns the dimensions and the filter for the
// opposite direction.
func (c *Component) graphReverseHandlerFunc(w http.ResponseWriter, req *http.Request) {
	var input graphReverseHandlerInput
	if err := httpserver.BindJSON(req, &input); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	if input.Dimensions == nil {
		input.Dimensions = []query.Column{}
	}
	if err := query.Columns(input.Dimensions).Validate(c.d.Schema); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	reversed, err := filter.Reverse(input.Filter, c.d.Schema)
	if err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": filter.HumanError(err)})
		return
	}
	query.Columns(input.Dimensions).Reverse(c.d.Schema)
	httpserver.WriteJSON(w, http.StatusOK, graphReverseHandlerOutput{
		Dimensions: input.Dimensions,
		Filter:     reversed,
	})
}
