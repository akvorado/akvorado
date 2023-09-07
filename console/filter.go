// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
	"akvorado/console/filter"
)

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

func (c *Component) filterValidateHandlerFunc(gc *gin.Context) {
	var input filterValidateHandlerInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	if strings.TrimSpace(input.Filter) == "" {
		gc.JSON(http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
		})
		return
	}
	got, err := filter.Parse("", []byte(input.Filter), filter.GlobalStore("meta", &filter.Meta{Schema: c.d.Schema}))
	if err == nil {
		gc.JSON(http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
			Parsed:  got.(string),
		})
		return
	}
	gc.JSON(http.StatusOK, filterValidateHandlerOutput{
		Message: filter.HumanError(err),
		Errors:  filter.AllErrors(err),
	})
}

// filterCompleteHandlerInput describes the input of the /filter/complete endpoint.
type filterCompleteHandlerInput struct {
	What   string `json:"what" binding:"required,oneof=column operator value"`
	Column string `json:"column" binding:"required_unless=What column"`
	Prefix string `json:"prefix"`
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

func (c *Component) filterCompleteHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	var input filterCompleteHandlerInput
	if err := gc.ShouldBindJSON(&input); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}

	completions := []filterCompletion{}
	switch input.What {
	case "column":
		// We use the schema directly.
		columns := []string{}
		for _, column := range c.d.Schema.Columns() {
			if column.Disabled {
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
			[]byte(fmt.Sprintf("%s ", input.Column)),
			filter.Entrypoint("ConditionExpr"),
			filter.GlobalStore("meta", &filter.Meta{Schema: c.d.Schema}))
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
			columnName := c.fixQueryColumnName(input.Column)
			sqlQuery := fmt.Sprintf(`
SELECT MACNumToString(%s) AS label
FROM flows
WHERE TimeReceived > date_sub(minute, 1, now())
AND positionCaseInsensitive(label, $1) >= 1
GROUP BY %s
ORDER BY COUNT(*) DESC
LIMIT 20`, columnName, columnName)
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery, input.Prefix); err != nil {
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
		case "dstcommunities":
			results := []struct {
				Label  string `ch:"label"`
				Detail string `ch:"detail"`
			}{}
			sqlQuery := `
SELECT label, detail FROM (
 SELECT
  'community' AS detail,
  concat(toString(bitShiftRight(c, 16)), ':', toString(bitAnd(c, 0xffff))) AS label
 FROM (
  SELECT arrayJoin(DstCommunities) AS c
  FROM flows
  WHERE TimeReceived > date_sub(minute, 1, now())
  GROUP BY c
  ORDER BY COUNT(*) DESC
 )

 UNION ALL

 SELECT
  'large community' AS detail,
  concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':', toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':', toString(bitAnd(c, 0xffffffff))) AS label
 FROM (
  SELECT arrayJoin(DstLargeCommunities) AS c
  FROM flows
  WHERE TimeReceived > date_sub(minute, 1, now())
  GROUP BY c
  ORDER BY COUNT(*) DESC
 )
)
WHERE startsWith(label, $1)
LIMIT 20`
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery, input.Prefix); err != nil {
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
			sqlQuery := fmt.Sprintf(`
SELECT label, detail FROM (
 SELECT concat('AS', toString(%s)) AS label, dictGet('asns', 'name', %s) AS detail, 1 AS rank
 FROM flows
 WHERE TimeReceived > date_sub(minute, 1, now())
 AND detail != ''
 AND positionCaseInsensitive(detail, $1) >= 1
 GROUP BY %s
 ORDER BY COUNT(*) DESC
 LIMIT 20
UNION DISTINCT
 SELECT concat('AS', toString(asn)) AS label, name AS detail, 2 AS rank
 FROM asns
 WHERE positionCaseInsensitive(name, $1) >= 1
 ORDER BY positionCaseInsensitive(name, $1) ASC, asn ASC
 LIMIT 20
) GROUP BY label, detail ORDER BY MIN(rank) ASC, MIN(rowNumberInBlock()) ASC LIMIT 20`,
				columnName, columnName, columnName)
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery, input.Prefix); err != nil {
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
		case "srcnetname", "dstnetname", "srcnetrole", "dstnetrole", "srcnetsite", "dstnetsite", "srcnetregion", "dstnetregion", "srcnettenant", "dstnettenant":
			attributeName := inputColumn[6:]
			results := []struct {
				Attribute string `ch:"attribute"`
			}{}
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, fmt.Sprintf(`
SELECT DISTINCT %s AS attribute
FROM networks
WHERE positionCaseInsensitive(%s, $1) >= 1
ORDER BY %s
LIMIT 20`, attributeName, attributeName, attributeName), input.Prefix); err != nil {
				c.r.Err(err).Msg("unable to query database")
				break
			}
			for _, result := range results {
				completions = append(completions, filterCompletion{
					Label:  result.Attribute,
					Detail: "network name",
					Quoted: true,
				})
			}
			input.Prefix = ""
		case "icmpv4", "icmpv6":
			columnName := c.fixQueryColumnName(input.Column)
			proto := 1
			if columnName == "ICMPv6" {
				proto = 58
			}
			results := []struct {
				Label string `ch:"label"`
			}{}
			err := c.d.ClickHouseDB.Conn.Select(ctx, &results, fmt.Sprintf(`
SELECT label FROM (
 SELECT %s AS label, 1 AS rank
 FROM flows
 WHERE TimeReceived > date_sub(minute, 1, now())
 AND Proto = %d
 AND positionCaseInsensitive(label, $1) >= 1
 GROUP BY %s
 ORDER BY COUNT(*) DESC
 LIMIT 20
UNION DISTINCT
 SELECT name AS label, 2 AS rank
 FROM icmp
 WHERE positionCaseInsensitive(label, $1) >= 1
 AND proto = %d
 ORDER BY positionCaseInsensitive(label, $1) ASC, type ASC, code ASC
 LIMIT 20
) GROUP BY label ORDER BY MIN(rank) ASC, MIN(rowNumberInBlock()) ASC LIMIT 20`,
				columnName, proto, columnName, proto),
				input.Prefix)
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
			sqlQuery := fmt.Sprintf(`
SELECT %s AS label
FROM exporters
WHERE positionCaseInsensitive(%s, $1) >= 1
GROUP BY %s
ORDER BY positionCaseInsensitive(%s, $1) ASC, %s ASC
LIMIT 20`, column, column, column, column, column)
			results := []struct {
				Label string `ch:"label"`
			}{}
			if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, sqlQuery, input.Prefix); err != nil {
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
				if err := c.d.ClickHouseDB.Conn.Select(ctx, &results, fmt.Sprintf(`
SELECT DISTINCT %s AS attribute
FROM flows
WHERE TimeReceived > date_sub(minute, 10, now()) AND startsWith(attribute, $1)
ORDER BY %s
LIMIT 20`, col.Name, col.Name), input.Prefix); err != nil {
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
	}

	filteredCompletions := []filterCompletion{}
	for _, completion := range completions {
		if strings.HasPrefix(strings.ToLower(completion.Label), strings.ToLower(input.Prefix)) {
			filteredCompletions = append(filteredCompletions, completion)
		}
	}
	gc.JSON(http.StatusOK, filterCompleteHandlerOutput{filteredCompletions})
}

func (c *Component) filterSavedListHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	user := gc.MustGet("user").(authentication.UserInformation).Login
	filters, err := c.d.Database.ListSavedFilters(ctx, user)
	if err != nil {
		c.r.Err(err).Msg("unable to list filters")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "unable to list filters"})
		return
	}
	gc.JSON(http.StatusOK, gin.H{"filters": filters})
}

func (c *Component) filterSavedDeleteHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	user := gc.MustGet("user").(authentication.UserInformation).Login
	id, err := strconv.ParseUint(gc.Param("id"), 10, 64)
	if err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": "bad ID format"})
		return
	}
	if err := c.d.Database.DeleteSavedFilter(ctx, database.SavedFilter{
		ID:   id,
		User: user,
	}); err != nil {
		// Assume this is because it is not found
		gc.JSON(http.StatusNotFound, gin.H{"message": "filter not found"})
		return
	}
	gc.JSON(http.StatusNoContent, nil)
}

func (c *Component) filterSavedAddHandlerFunc(gc *gin.Context) {
	ctx := c.t.Context(gc.Request.Context())
	user := gc.MustGet("user").(authentication.UserInformation).Login
	var filter database.SavedFilter
	if err := gc.ShouldBindJSON(&filter); err != nil {
		gc.JSON(http.StatusBadRequest, gin.H{"message": helpers.Capitalize(err.Error())})
		return
	}
	filter.User = user
	if err := c.d.Database.CreateSavedFilter(ctx, filter); err != nil {
		c.r.Err(err).Msg("cannot create saved filter")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create new filter"})
		return
	}
	gc.JSON(http.StatusNoContent, nil)
}
