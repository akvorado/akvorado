// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	netHTTP "net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"

	"akvorado/common/helpers"
)

func TestFilterHandlers(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT ExporterName AS label
FROM exporters
WHERE positionCaseInsensitive(ExporterName, $1) >= 1
GROUP BY ExporterName
ORDER BY positionCaseInsensitive(ExporterName, $1) ASC, ExporterName ASC
LIMIT 20`,
			"th2-").
		SetArg(1, []struct {
			Label string `ch:"label"`
		}{
			{"th2-router1"},
			{"th2-router2"},
			{"th2-router3"}}).
		Return(nil)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT DISTINCT name
FROM networks
WHERE positionCaseInsensitive(name, $1) >= 1
ORDER BY name
LIMIT 20`, "c").
		SetArg(1, []struct {
			Name string `ch:"name"`
		}{{"customer-1"}, {"customer-2"}, {"customer-3"}}).
		Return(nil)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT label, detail FROM (
 SELECT concat('AS', toString(DstAS)) AS label, dictGet('asns', 'name', DstAS) AS detail, 1 AS rank
 FROM flows
 WHERE TimeReceived > date_sub(minute, 1, now())
 AND detail != ''
 AND positionCaseInsensitive(detail, $1) >= 1
 GROUP BY DstAS
 ORDER BY COUNT(*) DESC
 LIMIT 20
UNION DISTINCT
 SELECT concat('AS', toString(asn)) AS label, name AS detail, 2 AS rank
 FROM asns
 WHERE positionCaseInsensitive(name, $1) >= 1
 ORDER BY positionCaseInsensitive(name, $1) ASC, asn ASC
 LIMIT 20
) GROUP BY label, detail ORDER BY MIN(rank) ASC, MIN(rowNumberInBlock()) ASC LIMIT 20`,
			"goog").
		SetArg(1, []struct {
			Label  string `ch:"label"`
			Detail string `ch:"detail"`
		}{
			{"AS15169", "Google"},
			{"AS16550", "Google Private Cloud"},
			{"AS16591", "Google Fiber"},
			{"AS19527", "Google"},
			{"AS26910", "GOOGLE-CLOUD-2"},
			{"AS36040", "Google"},
			{"AS36384", "Google"},
			{"AS36385", "Google IT"},
			{"AS36492", "Google"},
			{"AS36987", "Google Kenya"},
			{"AS41264", "Google Switzerland"},
		}).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL:       "/api/v0/console/filter/validate",
			JSONInput: gin.H{"filter": `InIfName = "Gi0/0/0/1"`},
			JSONOutput: gin.H{
				"message": "ok",
				"parsed":  `InIfName = 'Gi0/0/0/1'`},
		},
		{
			URL:       "/api/v0/console/filter/validate",
			JSONInput: gin.H{"filter": `InIfName = "`},
			JSONOutput: gin.H{
				"message": "at line 1, position 12: string literal not terminated",
				"errors": []gin.H{{
					"line":    1,
					"column":  12,
					"offset":  11,
					"message": "string literal not terminated",
				}},
			},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "column", "prefix": "dSt"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "DstAS", "detail": "column name", "quoted": false},
				{"label": "DstAddr", "detail": "column name", "quoted": false},
				{"label": "DstCountry", "detail": "column name", "quoted": false},
				{"label": "DstNetName", "detail": "column name", "quoted": false},
				{"label": "DstPort", "detail": "column name", "quoted": false},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "operator", "column": "ExporterName"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "!=", "detail": "comparison operator", "quoted": false},
				{"label": "=", "detail": "comparison operator", "quoted": false},
				{"label": "ILIKE", "detail": "comparison operator", "quoted": false},
				{"label": "IN (", "detail": "comparison operator", "quoted": false},
				{"label": "IUNLIKE", "detail": "comparison operator", "quoted": false},
				{"label": "LIKE", "detail": "comparison operator", "quoted": false},
				{"label": "NOTIN (", "detail": "comparison operator", "quoted": false},
				{"label": "UNLIKE", "detail": "comparison operator", "quoted": false},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "outifboundary"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "internal", "detail": "network boundary", "quoted": false},
				{"label": "external", "detail": "network boundary", "quoted": false},
				{"label": "undefined", "detail": "network boundary", "quoted": false},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "etype"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "IPv4", "detail": "ethernet type", "quoted": false},
				{"label": "IPv6", "detail": "ethernet type", "quoted": false},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "proto", "prefix": "I"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "ICMP", "detail": "protocol", "quoted": true},
				{"label": "IPv6-ICMP", "detail": "protocol", "quoted": true},
				{"label": "IPIP", "detail": "protocol", "quoted": true},
				{"label": "IGMP", "detail": "protocol", "quoted": true},
				{"label": "IPv4", "detail": "protocol", "quoted": true},
				{"label": "IPv6", "detail": "protocol", "quoted": true},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "exportername", "prefix": "th2-"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "th2-router1", "detail": "exporter name", "quoted": true},
				{"label": "th2-router2", "detail": "exporter name", "quoted": true},
				{"label": "th2-router3", "detail": "exporter name", "quoted": true},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstAS", "prefix": "goog"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "AS15169", "detail": "Google", "quoted": false},
				{"label": "AS16550", "detail": "Google Private Cloud", "quoted": false},
				{"label": "AS16591", "detail": "Google Fiber", "quoted": false},
				{"label": "AS19527", "detail": "Google", "quoted": false},
				{"label": "AS26910", "detail": "GOOGLE-CLOUD-2", "quoted": false},
				{"label": "AS36040", "detail": "Google", "quoted": false},
				{"label": "AS36384", "detail": "Google", "quoted": false},
				{"label": "AS36385", "detail": "Google IT", "quoted": false},
				{"label": "AS36492", "detail": "Google", "quoted": false},
				{"label": "AS36987", "detail": "Google Kenya", "quoted": false},
				{"label": "AS41264", "detail": "Google Switzerland", "quoted": false},
			}},
		}, {
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "srcnetName", "prefix": "c"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "customer-1", "detail": "network name", "quoted": true},
				{"label": "customer-2", "detail": "network name", "quoted": true},
				{"label": "customer-3", "detail": "network name", "quoted": true},
			}},
		}, {
			Description: "list, no filters",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  200,
			JSONOutput:  gin.H{"filters": []gin.H{}},
		}, {
			Description: "store one filter",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  204,
			JSONInput: gin.H{
				"description": "test 1",
				"content":     "InIfBoundary = external",
			},
			ContentType: "application/json; charset=utf-8",
		}, {
			Description: "list stored filters",
			URL:         "/api/v0/console/filter/saved",
			JSONOutput: gin.H{"filters": []gin.H{
				{
					"id":          1,
					"shared":      false,
					"user":        "__default",
					"description": "test 1",
					"content":     "InIfBoundary = external",
				},
			}},
		}, {
			Description: "list stored filters as another user",
			URL:         "/api/v0/console/filter/saved",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("Remote-User", "alfred")
				return headers
			}(),
			JSONOutput: gin.H{"filters": []gin.H{}},
		}, {
			Description: "delete stored filter as another user",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/1",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("Remote-User", "alfred")
				return headers
			}(),
			StatusCode: 404,
			JSONOutput: gin.H{"message": "filter not found"},
		}, {
			Description: "delete stored filter",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/1",
			StatusCode:  204,
			ContentType: "application/json; charset=utf-8",
		}, {
			Description: "delete stored filter with invalid ID",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/kjgdfhgh",
			StatusCode:  400,
			JSONOutput:  gin.H{"message": "bad ID format"},
		}, {
			Description: "list stored filter after delete",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  200,
			JSONOutput:  gin.H{"filters": []gin.H{}},
		},
	})
}
