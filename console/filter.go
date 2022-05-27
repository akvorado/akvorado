package console

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/console/filter"
)

// filterValidateHandlerInput describes the input for the /filter/validate endpoint.
type filterValidateHandlerInput struct {
	Filter string `json:"filter" binding:"required"`
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

	got, err := filter.Parse("", []byte(input.Filter))
	if err == nil {
		gc.JSON(http.StatusOK, filterValidateHandlerOutput{
			Message: "ok",
			Parsed:  got.(string),
		})
		return
	}
	gc.JSON(http.StatusBadRequest, filterValidateHandlerOutput{
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
		_, err := filter.Parse("", []byte{}, filter.Entrypoint("ConditionExpr"))
		if err != nil {
			for _, candidate := range filter.Expected(err) {
				if !strings.HasSuffix(candidate, `"i`) {
					continue
				}
				candidate = candidate[1 : len(candidate)-2]
				completions = append(completions, filterCompletion{
					Label:  candidate,
					Detail: "column name",
				})
			}
		}
	case "operator":
		_, err := filter.Parse("",
			[]byte(fmt.Sprintf("%s ", input.Column)),
			filter.Entrypoint("ConditionExpr"))
		if err != nil {
			for _, candidate := range filter.Expected(err) {
				if !strings.HasPrefix(candidate, `"`) {
					continue
				}
				candidate = strings.TrimSuffix(
					strings.TrimSuffix(candidate[1:len(candidate)-1], `"i`),
					`"`)
				if candidate != "--" && candidate != "/*" {
					completions = append(completions, filterCompletion{
						Label:  candidate,
						Detail: "condition operator",
					})
				}
			}
		}
	case "value":
		var column, detail string
		switch strings.ToLower(input.Column) {
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
			// Do not complete from Clickhouse, we want a subset of options
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
		case "srcas", "dstas":
			results := []struct {
				Label  string `ch:"label"`
				Detail string `ch:"detail"`
			}{}
			sqlQuery := `
SELECT label, detail FROM (
 SELECT concat('AS', toString(SrcAS)) AS label, dictGet('asns', 'name', SrcAS) AS detail, 1 AS rank
 FROM flows
 WHERE TimeReceived > date_sub(minute, 1, now())
 AND detail != ''
 AND positionCaseInsensitive(detail, $1) >= 1
 GROUP BY SrcAS
 ORDER BY SUM(Bytes) DESC
 LIMIT 20
UNION DISTINCT
 SELECT concat('AS', toString(asn)) AS label, name AS detail, 2 AS rank
 FROM asns
 WHERE positionCaseInsensitive(name, $1) >= 1
 ORDER BY positionCaseInsensitive(name, $1) ASC, asn ASC
 LIMIT 20
) ORDER BY rank ASC, rowNumberInBlock() ASC LIMIT 20`
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
		case "exportername":
			column = "ExporterName"
			detail = "exporter name"
		case "exportergroup":
			column = "ExporterGroup"
			detail = "exporter group"
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
	}
	filteredCompletions := []filterCompletion{}
	for _, completion := range completions {
		if strings.HasPrefix(strings.ToLower(completion.Label), strings.ToLower(input.Prefix)) {
			filteredCompletions = append(filteredCompletions, completion)
		}
	}
	gc.JSON(http.StatusOK, filterCompleteHandlerOutput{filteredCompletions})
	return
}
