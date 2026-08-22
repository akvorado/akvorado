// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"akvorado/common/constants"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/authentication"
	"akvorado/console/database"
	"akvorado/console/filter"
	"akvorado/console/query"
)

// recentFlows keeps the flows received during the last minutes. Completion
// proposes what the exporters send right now.
func recentFlows(minutes uint64) sb.Expr {
	return sb.Op(sb.Column("TimeReceived"), ">",
		sb.Function("date_sub", sb.Column("minute"), sb.Uint(minutes), sb.Function("now")))
}

// prefixPosition tells where the prefix the user typed appears in an
// expression, whatever the case. It is 0 when it does not appear at all.
func prefixPosition(expr sb.Expr, prefix string) sb.Expr {
	return sb.Function("positionCaseInsensitive", expr, sb.String(prefix))
}

// matchPrefix keeps the rows where the prefix the user typed appears.
func matchPrefix(expr sb.Expr, prefix string) sb.Expr {
	return sb.Op(prefixPosition(expr, prefix), ">=", sb.Uint(1))
}

// mostUsedFirst sorts the rows on how often they appear in the flows.
func mostUsedFirst() sb.OrderItem {
	return sb.Order(sb.Function("COUNT", sb.Star())).Desc()
}

// filterValidateHandlerInput describes the input for the /filter/validate endpoint.
type filterValidateHandlerInput struct {
	Filter string `json:"filter"`
}

// filterValidateHandlerOutput describes the output for the /filter/validate endpoint.
type filterValidateHandlerOutput struct {
	Message string        `json:"message"`
	Parsed  string        `json:"parsed,omitempty"`
	Errors  filter.Errors `json:"errors,omitempty"`
}

func (c *Component) filterValidateHandlerFunc(w http.ResponseWriter, req *http.Request) {
	var input filterValidateHandlerInput
	if err := httpserver.BindJSON(req, &input); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}

	if strings.TrimSpace(input.Filter) == "" {
		httpserver.WriteJSON(w, http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
		})
		return
	}
	got, err := filter.Parse("", []byte(input.Filter),
		filter.GlobalStore("meta", &filter.Meta{
			Schema:   c.d.Schema,
			Database: c.d.ClickHouseDB.DatabaseName(),
		}))
	if err == nil {
		httpserver.WriteJSON(w, http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
			Parsed:  got.(sb.Expr).String(),
		})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, filterValidateHandlerOutput{
		Message: filter.HumanError(err),
		Errors:  filter.AllErrors(err),
	})
}

// filterCompleteHandlerInput describes the input of the /filter/complete endpoint.
type filterCompleteHandlerInput struct {
	What     string `json:"what" validate:"required,oneof=column operator value"`
	Column   string `json:"column" validate:"required_unless=What column"`
	Operator string `json:"operator"`
	Prefix   string `json:"prefix"`
	Limit    int    `json:"limit"`
}

// filterCompleteHandlerOutput describes the output of the /filter/complete endpoint.
type filterCompleteHandlerOutput struct {
	Completions []filterCompletion `json:"completions"`
}
type filterCompletion struct {
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	Quoted bool   `json:"quoted"` // should the return value be quoted?
}

func (c *Component) filterCompleteHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	input := filterCompleteHandlerInput{Limit: 20}
	if err := httpserver.BindJSON(req, &input); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}

	completions := []filterCompletion{}
	switch input.What {
	case "column":
		// We use the schema directly.
		columns := []string{}
		for _, column := range c.d.Schema.Columns() {
			if column.Disabled || column.ParserType == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(column.Name), strings.ToLower(input.Prefix)) {
				columns = append(columns, column.Name)
			}
		}
		sort.Strings(columns)
		for _, column := range columns {
			completions = append(completions, filterCompletion{
				Label:  column,
				Detail: "column name",
			})
		}
	case "operator":
		_, err := filter.Parse("",
			fmt.Appendf(nil, "%s ", input.Column),
			filter.Entrypoint("ConditionExpr"),
			filter.GlobalStore("meta", &filter.Meta{
				Schema:   c.d.Schema,
				Database: c.d.ClickHouseDB.DatabaseName(),
			}))
		if err != nil {
			for _, candidate := range filter.Expected(err) {
				if !strings.HasPrefix(candidate, `"`) {
					continue
				}
				candidate = strings.TrimSuffix(
					strings.TrimSuffix(candidate[1:len(candidate)-1], `"i`),
					`"`)
				if candidate != "--" && candidate != "/*" {
					if candidate == "IN" || candidate == "NOTIN" {
						candidate = candidate + " ("
					}
					completions = append(completions, filterCompletion{
						Label:  candidate,
						Detail: "comparison operator",
					})
				}
			}
		}
	case "value":
		var column, detail string
		inputColumn := strings.ToLower(input.Column)
		otherColumns := c.filterComparableColumns(input.Column, input.Operator, input.Prefix)
		switch inputColumn {
		case "inifboundary", "outifboundary":
			completions = append(completions, filterCompletion{
				Label:  "internal",
				Detail: "network boundary",
			}, filterCompletion{
				Label:  "external",
				Detail: "network boundary",
			}, filterCompletion{
				Label:  "undefined",
				Detail: "network boundary",
			})
		case "flowdirection":
			completions = append(completions, filterCompletion{
				Label:  "ingress",
				Detail: "flow direction",
			}, filterCompletion{
				Label:  "egress",
				Detail: "flow direction",
			}, filterCompletion{
				Label:  "undefined",
				Detail: "flow direction",
			})
		case "etype":
			completions = append(completions, filterCompletion{
				Label:  "IPv4",
				Detail: "ethernet type",
			}, filterCompletion{
				Label:  "IPv6",
				Detail: "ethernet type",
			})
		case "proto":
			// Do not complete from ClickHouse, we want a subset of options
			completions = append(completions,
				filterCompletion{"TCP", "protocol", true},
				filterCompletion{"UDP", "protocol", true},
				filterCompletion{"SCTP", "protocol", true},
				filterCompletion{"ICMP", "protocol", true},
				filterCompletion{"IPv6-ICMP", "protocol", true},
				filterCompletion{"GRE", "protocol", true},
				filterCompletion{"ESP", "protocol", true},
				filterCompletion{"AH", "protocol", true},
				filterCompletion{"IPIP", "protocol", true},
				filterCompletion{"VRRP", "protocol", true},
				filterCompletion{"L2TP", "protocol", true},
				filterCompletion{"IGMP", "protocol", true},
				filterCompletion{"PIM", "protocol", true},
				filterCompletion{"IPv4", "protocol", true},
				filterCompletion{"IPv6", "protocol", true})
		case "srcmac", "dstmac":
			results := []struct {
				Label string `ch:"label"`
			}{}
			column := sb.Column(c.fixQueryColumnName(input.Column))
			sqlQuery := sb.Select(sb.Alias(sb.Function("MACNumToString", column), "label")).
				From(sb.Table("flows")).
				Where(sb.And(
					recentFlows(1),
					matchPrefix(sb.Column("label"), input.Prefix))).
				GroupBy(column).
				OrderBy(mostUsedFirst()).
				Limit(input.Limit).
				String()
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: "MAC address",
					Quoted: false,
				})
			}
			input.Prefix = "" // We have handled this internally
		case "srccommunities", "dstcommunities":
			results := []struct {
				Label  string `ch:"label"`
				Detail string `ch:"detail"`
			}{}
			columnNamePrefix := c.fixQueryColumnName(input.Column)[:3]
			// Each community of the recent flows, taken out of the arrays.
			unrolled := func(column string) *sb.Query {
				return sb.Select(sb.Alias(
					sb.Function("arrayJoin", sb.Function("arrayJoin", sb.Column(column))), "c")).
					From(sb.Table("flows")).
					Where(recentFlows(1)).
					GroupBy(sb.Column("c")).
					OrderBy(mostUsedFirst())
			}
			communities := sb.Select(
				sb.Alias(sb.String("community"), "detail"),
				sb.Alias(query.CommunityToString(sb.Column("c")), "label")).
				FromSelect(unrolled(fmt.Sprintf("%sCommunities", columnNamePrefix)))
			largeCommunities := sb.Select(
				sb.Alias(sb.String("large community"), "detail"),
				sb.Alias(query.LargeCommunityToString(sb.Column("c")), "label")).
				FromSelect(unrolled(fmt.Sprintf("%sLargeCommunities", columnNamePrefix)))
			sqlQuery := sb.Select(sb.Columns("label", "detail")...).
				FromSelect(communities.UnionAll(largeCommunities)).
				Where(sb.Function("startsWith", sb.Column("label"), sb.String(input.Prefix))).
				Limit(input.Limit).
				String()
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: result.Detail,
					Quoted: false,
				})
			}
			input.Prefix = ""
		case "srcas", "dstas", "dst1stas", "dst2ndas", "dst3rdas", "dstaspath":
			results := []struct {
				Label  string `ch:"label"`
				Detail string `ch:"detail"`
			}{}
			columnName := c.fixQueryColumnName(input.Column)
			if columnName == "DstASPath" {
				columnName = "DstAS"
			}
			asns := c.dictionary(schema.DictionaryASNs)
			column := sb.Column(columnName)
			// The AS numbers seen in the recent flows, the most used first.
			fromFlows := sb.Select(
				sb.Alias(sb.Function("concat",
					sb.String("AS"), sb.Function("toString", column)), "label"),
				sb.Alias(sb.Function("dictGet",
					sb.String(asns.String()), sb.String("name"), column), "detail"),
				sb.Alias(sb.Uint(1), "rank")).
				From(sb.Table("flows")).
				Where(sb.And(
					recentFlows(1),
					sb.Op(sb.Column("detail"), "!=", sb.String("")),
					matchPrefix(sb.Column("detail"), input.Prefix))).
				GroupBy(column).
				OrderBy(mostUsedFirst()).
				Limit(input.Limit)
			// The other AS numbers of the dictionary, whether they were seen or not.
			fromDictionary := sb.Select(
				sb.Alias(sb.Function("concat",
					sb.String("AS"), sb.Function("toString", sb.Column("asn"))), "label"),
				sb.Alias(sb.Column("name"), "detail"),
				sb.Alias(sb.Uint(2), "rank")).
				From(asns).
				Where(matchPrefix(sb.Column("name"), input.Prefix)).
				OrderBy(
					sb.Order(prefixPosition(sb.Column("name"), input.Prefix)),
					sb.Order(sb.Column("asn"))).
				Limit(input.Limit)
			sqlQuery := sb.Select(sb.Columns("label", "detail")...).
				FromSelect(fromFlows.UnionDistinct(fromDictionary)).
				GroupBy(sb.Columns("label", "detail")...).
				OrderBy(
					sb.Order(sb.Function("MIN", sb.Column("rank"))),
					sb.Order(sb.Function("MIN", sb.Function("rowNumberInBlock")))).
				Limit(input.Limit).
				String()
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: result.Detail,
					Quoted: false,
				})
			}
			input.Prefix = "" // We have handled this internally
		case "srcport", "dstport":
			results := []struct {
				Label  string `ch:"label"`
				Detail string `ch:"detail"`
			}{}
			column := sb.Column(c.fixQueryColumnName(input.Column))
			tcp := c.dictionary(schema.DictionaryTCP)
			udp := c.dictionary(schema.DictionaryUDP)
			portName := func(dictionary sb.TableName) sb.Expr {
				return sb.Function("dictGet",
					sb.String(dictionary.String()), sb.String("name"), column)
			}
			// The ports seen in the recent flows, the most used first.
			fromFlows := sb.Select(
				sb.Alias(sb.Function("toString", column), "label"),
				sb.Alias(sb.Function("if",
					sb.Op(sb.Column("Proto"), "=", sb.Uint(constants.ProtoTCP)),
					portName(tcp),
					portName(udp)), "detail"),
				sb.Alias(sb.Uint(1), "rank"),
				sb.Alias(sb.Function("COUNT", sb.Star()), "c")).
				From(sb.Table("flows")).
				Where(sb.And(
					sb.Op(sb.Column("Proto"), "IN", sb.Tuple(
						sb.Uint(constants.ProtoTCP), sb.Uint(constants.ProtoUDP))),
					recentFlows(1),
					sb.Op(sb.Column("detail"), "!=", sb.String("")),
					matchPrefix(sb.Column("detail"), input.Prefix))).
				GroupBy(column, sb.Column("Proto")).
				OrderBy(mostUsedFirst()).
				Limit(input.Limit)
			// The other ports of the dictionaries, whether they were seen or not.
			knownPorts := sb.Select(sb.Columns("port", "name")...).From(tcp).
				UnionDistinct(sb.Select(sb.Columns("port", "name")...).From(udp))
			fromDictionaries := sb.Select(
				sb.Alias(sb.Function("toString", sb.Column("port")), "label"),
				sb.Alias(sb.Column("name"), "detail"),
				sb.Alias(sb.Uint(2), "rank"),
				sb.Alias(sb.Uint(0), "c")).
				FromSelect(knownPorts).
				Where(matchPrefix(sb.Column("name"), input.Prefix)).
				OrderBy(
					sb.Order(prefixPosition(sb.Column("name"), input.Prefix)),
					sb.Order(sb.Column("port"))).
				Limit(input.Limit)
			sqlQuery := sb.Select(sb.Columns("label", "detail")...).
				FromSelect(fromFlows.UnionDistinct(fromDictionaries)).
				GroupBy(sb.Columns("rank", "label", "detail")...).
				OrderBy(
					sb.Order(sb.Column("rank")),
					sb.Order(sb.Function("MAX", sb.Column("c"))).Desc(),
					sb.Order(sb.Function("MIN", sb.Function("rowNumberInBlock")))).
				Limit(input.Limit).
				String()
			if err := c.d.ClickHouseDB.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: result.Detail,
					Quoted: false,
				})
			}
			input.Prefix = ""
		case "srcnetname", "dstnetname", "srcnetrole", "dstnetrole", "srcnetsite", "dstnetsite", "srcnetregion", "dstnetregion", "srcnettenant", "dstnettenant":
			results := []struct {
				Label string `ch:"label"`
			}{}
			column := sb.Column(c.fixQueryColumnName(input.Column))
			// The attributes seen in the recent flows, the most used first.
			sqlQuery := sb.Select(sb.Alias(column, "label")).
				From(sb.Table("flows")).
				Where(sb.And(
					recentFlows(10),
					sb.Op(sb.Column("label"), "!=", sb.String("")),
					matchPrefix(sb.Column("label"), input.Prefix))).
				GroupBy(column).
				OrderBy(
					sb.Order(prefixPosition(sb.Column("label"), input.Prefix)),
					mostUsedFirst()).
				Limit(input.Limit).
				String()
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: fmt.Sprintf("network %s", inputColumn[6:]),
					Quoted: true,
				})
			}
			input.Prefix = ""
		case "icmpv4", "icmpv6":
			columnName := c.fixQueryColumnName(input.Column)
			proto := uint64(constants.ProtoICMPv4)
			if columnName == "ICMPv6" {
				proto = constants.ProtoICMPv6
			}
			results := []struct {
				Label string `ch:"label"`
			}{}
			column := sb.Column(columnName)
			// The ICMP types seen in the recent flows, the most used first.
			fromFlows := sb.Select(
				sb.Alias(column, "label"),
				sb.Alias(sb.Uint(1), "rank")).
				From(sb.Table("flows")).
				Where(sb.And(
					recentFlows(1),
					sb.Op(sb.Column("Proto"), "=", sb.Uint(proto)),
					matchPrefix(sb.Column("label"), input.Prefix))).
				GroupBy(column).
				OrderBy(mostUsedFirst()).
				Limit(input.Limit)
			// The other ICMP types of the dictionary, whether they were seen or not.
			fromDictionary := sb.Select(
				sb.Alias(sb.Column("name"), "label"),
				sb.Alias(sb.Uint(2), "rank")).
				From(c.dictionary(schema.DictionaryICMP)).
				Where(sb.And(
					matchPrefix(sb.Column("label"), input.Prefix),
					sb.Op(sb.Column("proto"), "=", sb.Uint(proto)))).
				OrderBy(
					sb.Order(prefixPosition(sb.Column("label"), input.Prefix)),
					sb.Order(sb.Column("type")),
					sb.Order(sb.Column("code"))).
				Limit(input.Limit)
			sqlQuery := sb.Select(sb.Column("label")).
				FromSelect(fromFlows.UnionDistinct(fromDictionary)).
				GroupBy(sb.Column("label")).
				OrderBy(
					sb.Order(sb.Function("MIN", sb.Column("rank"))),
					sb.Order(sb.Function("MIN", sb.Function("rowNumberInBlock")))).
				Limit(input.Limit).
				String()
			err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery)
			if err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: columnName,
					Quoted: true,
				})
			}
			input.Prefix = ""
		case "exportername", "exportergroup", "exporterrole", "exportersite", "exporterregion", "exportertenant":
			column = c.fixQueryColumnName(inputColumn)
			detail = fmt.Sprintf("exporter %s", inputColumn[8:])
		case "inifname", "outifname":
			column = "IfName"
			detail = "interface name"
		case "inifdescription", "outifdescription":
			column = "IfDescription"
			detail = "interface description"
		case "inifconnectivity", "outifconnectivity":
			column = "IfConnectivity"
			detail = "connectivity type"
		case "inifprovider", "outifprovider":
			column = "IfProvider"
			detail = "provider name"
		}
		if column != "" {
			// Query "exporter" table
			name := sb.Column(column)
			sqlQuery := sb.Select(sb.Alias(name, "label")).
				From(sb.Table("exporters")).
				Where(matchPrefix(name, input.Prefix)).
				GroupBy(name).
				OrderBy(
					sb.Order(prefixPosition(name, input.Prefix)),
					sb.Order(name)).
				Limit(input.Limit).
				String()
			results := []struct {
				Label string `ch:"label"`
			}{}
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Label,
					Detail: detail,
					Quoted: true,
				})
			}
			input.Prefix = ""
		}

		// Custom columns are handled here
		for _, col := range c.d.Schema.Columns() {
			// First filter out custom columns, iterate and try to match
			if col.Key >= schema.ColumnLast {
				if inputColumn != strings.ToLower(col.Name) || col.ParserType != "string" {
					continue
				}
				results := []struct {
					Attribute string `ch:"attribute"`
				}{}
				name := sb.Column(col.Name)
				sqlQuery := sb.Select(sb.Alias(name, "attribute")).
					Distinct().
					From(sb.Table("flows")).
					Where(sb.And(
						recentFlows(10),
						sb.Function("startsWith",
							sb.Column("attribute"), sb.String(input.Prefix)))).
					OrderBy(sb.Order(name)).
					Limit(input.Limit).
					String()
				if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery); err != nil {
					c.r.Err(err).Msg("unable to query database")
					break
				}
				for _, result := range results {
					completions = append(completions, filterCompletion{
						Label:  result.Attribute,
						Quoted: true,
					})
				}
			}
		}

		completions = append(completions, otherColumns...)
	}

	filteredCompletions := []filterCompletion{}
	for _, completion := range completions {
		if strings.HasPrefix(strings.ToLower(completion.Label), strings.ToLower(input.Prefix)) {
			filteredCompletions = append(filteredCompletions, completion)
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, filterCompleteHandlerOutput{filteredCompletions})
}

// filterComparableColumns returns the columns which can be used on the right
// side of the provided column and operator.
func (c *Component) filterComparableColumns(name, operator, prefix string) []filterCompletion {
	var parserType string
	for _, column := range c.d.Schema.Columns() {
		if strings.EqualFold(name, column.Name) {
			parserType = column.ParserType
			break
		}
	}
	var operators []string
	switch parserType {
	case "uint":
		operators = []string{"=", "!=", "<", "<=", ">", ">="}
	case "asn", "string":
		operators = []string{"=", "!="}
	default:
		return nil
	}
	if !slices.Contains(operators, operator) {
		return nil
	}

	names := []string{}
	for _, column := range c.d.Schema.Columns() {
		if column.Disabled || column.ParserType != parserType || strings.EqualFold(name, column.Name) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(column.Name), strings.ToLower(prefix)) {
			names = append(names, column.Name)
		}
	}
	sort.Strings(names)
	completions := make([]filterCompletion, 0, len(names))
	for _, name := range names {
		completions = append(completions, filterCompletion{
			Label:  name,
			Detail: "column name",
		})
	}
	return completions
}

func (c *Component) filterSavedListHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	user := authentication.UserFromContext(req.Context()).Login
	filters, err := c.d.Database.ListSavedFilters(ctx, user)
	if err != nil {
		c.r.Err(err).Msg("unable to list filters")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "unable to list filters"})
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, helpers.M{"filters": filters})
}

func (c *Component) filterSavedDeleteHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	user := authentication.UserFromContext(req.Context()).Login
	id, err := strconv.ParseUint(req.PathValue("id"), 10, 64)
	if err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": "bad ID format"})
		return
	}
	if err := c.d.Database.DeleteSavedFilter(ctx, database.SavedFilter{
		ID:   id,
		User: user,
	}); err != nil {
		// Assume this is because it is not found
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "filter not found"})
		return
	}
	httpserver.WriteJSON(w, http.StatusNoContent, nil)
}

func (c *Component) filterSavedAddHandlerFunc(w http.ResponseWriter, req *http.Request) {
	ctx := c.t.Context(req.Context())
	user := authentication.UserFromContext(req.Context()).Login
	var filter database.SavedFilter
	if err := httpserver.BindJSON(req, &filter); err != nil {
		httpserver.WriteJSON(w, http.StatusBadRequest, helpers.M{"message": helpers.Capitalize(err.Error())})
		return
	}
	filter.User = user
	if err := c.d.Database.CreateSavedFilter(ctx, filter); err != nil {
		c.r.Err(err).Msg("cannot create saved filter")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "cannot create new filter"})
		return
	}
	httpserver.WriteJSON(w, http.StatusNoContent, nil)
}
