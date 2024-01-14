// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"akvorado/common/helpers"
	"akvorado/common/schema"
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
			{"th2-router3"},
		}).
		Return(nil)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT DISTINCT name AS attribute
FROM networks
WHERE positionCaseInsensitive(name, $1) >= 1
ORDER BY name
LIMIT 20`, "c").
		SetArg(1, []struct {
			Attribute string `ch:"attribute"`
		}{{"customer-1"}, {"customer-2"}, {"customer-3"}}).
		Return(nil)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT DISTINCT role AS attribute
FROM networks
WHERE positionCaseInsensitive(role, $1) >= 1
ORDER BY role
LIMIT 20`, "c").
		SetArg(1, []struct {
			Attribute string `ch:"attribute"`
		}{{"customer"}}).
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
		Return(nil).
		MinTimes(2).MaxTimes(2)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
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
LIMIT 20`, "6540").
		SetArg(1, []struct {
			Label  string `ch:"label"`
			Detail string `ch:"detail"`
		}{
			{"65401:10", "community"},
			{"65401:12", "community"},
			{"65401:13", "community"},
			{"65402:200:100", "large community"},
		}).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:       "/api/v0/console/filter/validate",
			JSONInput: gin.H{"filter": `InIfName = "Gi0/0/0/1"`},
			JSONOutput: gin.H{
				"message": "ok",
				"parsed":  `InIfName = 'Gi0/0/0/1'`,
			},
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "column", "prefix": "dSta"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "DstAS", "detail": "column name", "quoted": false},
				{"label": "DstASPath", "detail": "column name", "quoted": false},
				{"label": "DstAddr", "detail": "column name", "quoted": false},
			}},
		},
		{
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "outifboundary"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "internal", "detail": "network boundary", "quoted": false},
				{"label": "external", "detail": "network boundary", "quoted": false},
				{"label": "undefined", "detail": "network boundary", "quoted": false},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "etype"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "IPv4", "detail": "ethernet type", "quoted": false},
				{"label": "IPv6", "detail": "ethernet type", "quoted": false},
			}},
		},
		{
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "exportername", "prefix": "th2-"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "th2-router1", "detail": "exporter name", "quoted": true},
				{"label": "th2-router2", "detail": "exporter name", "quoted": true},
				{"label": "th2-router3", "detail": "exporter name", "quoted": true},
			}},
		},
		{
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstcommunities", "prefix": "6540"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "65401:10", "detail": "community", "quoted": false},
				{"label": "65401:12", "detail": "community", "quoted": false},
				{"label": "65401:13", "detail": "community", "quoted": false},
				{"label": "65402:200:100", "detail": "large community", "quoted": false},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstASpath", "prefix": "goog"},
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "srcnetName", "prefix": "c"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "customer-1", "detail": "network name", "quoted": true},
				{"label": "customer-2", "detail": "network name", "quoted": true},
				{"label": "customer-3", "detail": "network name", "quoted": true},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstnetRole", "prefix": "c"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "customer", "detail": "network name", "quoted": true},
			}},
		},
		{
			Description: "list, no filters",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  200,
			JSONOutput:  gin.H{"filters": []gin.H{}},
		},
		{
			Description: "store one filter",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  204,
			JSONInput: gin.H{
				"description": "test 1",
				"content":     "InIfBoundary = external",
			},
			ContentType: "application/json; charset=utf-8",
		},
		{
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
		},
		{
			Description: "list stored filters as another user",
			URL:         "/api/v0/console/filter/saved",
			Header: func() http.Header {
				headers := make(http.Header)
				headers.Add("Remote-User", "alfred")
				return headers
			}(),
			JSONOutput: gin.H{"filters": []gin.H{}},
		},
		{
			Description: "delete stored filter as another user",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/1",
			Header: func() http.Header {
				headers := make(http.Header)
				headers.Add("Remote-User", "alfred")
				return headers
			}(),
			StatusCode: 404,
			JSONOutput: gin.H{"message": "filter not found"},
		},
		{
			Description: "delete stored filter",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/1",
			StatusCode:  204,
			ContentType: "application/json; charset=utf-8",
		},
		{
			Description: "delete stored filter with invalid ID",
			Method:      "DELETE",
			URL:         "/api/v0/console/filter/saved/kjgdfhgh",
			StatusCode:  400,
			JSONOutput:  gin.H{"message": "bad ID format"},
		},
		{
			Description: "list stored filter after delete",
			URL:         "/api/v0/console/filter/saved",
			StatusCode:  200,
			JSONOutput:  gin.H{"filters": []gin.H{}},
		},
	})
}

func TestFilterHandlersMore(t *testing.T) {
	c, h, mockConn, _ := NewMock(t, DefaultConfiguration())
	c.d.Schema = schema.NewMock(t).EnableAllColumns()

	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT MACNumToString(SrcMAC) AS label
FROM flows
WHERE TimeReceived > date_sub(minute, 1, now())
AND positionCaseInsensitive(label, $1) >= 1
GROUP BY SrcMAC
ORDER BY COUNT(*) DESC
LIMIT 20`, "11:").
		SetArg(1, []struct {
			Label string `ch:"label"`
		}{
			{"11:22:33:44:55:66"},
			{"11:33:33:44:55:66"},
			{"11:ff:33:44:55:66"},
		}).
		Return(nil)
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT label FROM (
 SELECT ICMPv6 AS label, 1 AS rank
 FROM flows
 WHERE TimeReceived > date_sub(minute, 1, now())
 AND Proto = 58
 AND positionCaseInsensitive(label, $1) >= 1
 GROUP BY ICMPv6
 ORDER BY COUNT(*) DESC
 LIMIT 20
UNION DISTINCT
 SELECT name AS label, 2 AS rank
 FROM icmp
 WHERE positionCaseInsensitive(label, $1) >= 1
 AND proto = 58
 ORDER BY positionCaseInsensitive(label, $1) ASC, type ASC, code ASC
 LIMIT 20
) GROUP BY label ORDER BY MIN(rank) ASC, MIN(rowNumberInBlock()) ASC LIMIT 20`, "echo").
		SetArg(1, []struct {
			Label string `ch:"label"`
		}{
			{"echo-request"},
			{"echo-reply"},
		}).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "srcMAC", "prefix": "11:"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "11:22:33:44:55:66", "detail": "MAC address", "quoted": false},
				{"label": "11:33:33:44:55:66", "detail": "MAC address", "quoted": false},
				{"label": "11:ff:33:44:55:66", "detail": "MAC address", "quoted": false},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "icmpv6", "prefix": "echo"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "echo-request", "detail": "ICMPv6", "quoted": true},
				{"label": "echo-reply", "detail": "ICMPv6", "quoted": true},
			}},
		},
	})
}

func TestFilterHandlersCustomDict(t *testing.T) {
	c, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT DISTINCT DstAddrRole AS attribute
FROM flows
WHERE TimeReceived > date_sub(minute, 10, now()) AND startsWith(attribute, $1)
ORDER BY DstAddrRole
LIMIT 20`, "").
		SetArg(1, []struct {
			Attribute string `ch:"attribute"`
		}{{"a-role"}, {"b-role"}, {"c-role"}}).
		Return(nil)

	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT DISTINCT DstAddrRole AS attribute
FROM flows
WHERE TimeReceived > date_sub(minute, 10, now()) AND startsWith(attribute, $1)
ORDER BY DstAddrRole
LIMIT 20`, "a").
		SetArg(1, []struct {
			Attribute string `ch:"attribute"`
		}{{"a-role"}}).
		Return(nil)

	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "SrcAddr", Type: "String"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "String", Label: "DimensionAttribute"},
			{Name: "role", Type: "String"},
		},
		Source:     "test.csv",
		Dimensions: []string{"SrcAddr", "DstAddr"},
	}

	s, _ := schema.New(config)
	c.d.Schema = s

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "column", "prefix": "dSta"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "DstAS", "detail": "column name", "quoted": false},
				{"label": "DstASPath", "detail": "column name", "quoted": false},
				{"label": "DstAddr", "detail": "column name", "quoted": false},
				{"label": "DstAddrDimensionAttribute", "detail": "column name", "quoted": false},
				{"label": "DstAddrRole", "detail": "column name", "quoted": false},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "operator", "column": "DstAddrRole"},
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
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstaddrrole"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "a-role", "quoted": true},
				{"label": "b-role", "quoted": true},
				{"label": "c-role", "quoted": true},
			}},
		},
		{
			URL:        "/api/v0/console/filter/complete",
			StatusCode: 200,
			JSONInput:  gin.H{"what": "value", "column": "dstaddrrole", "prefix": "a"},
			JSONOutput: gin.H{"completions": []gin.H{
				{"label": "a-role", "quoted": true},
			}},
		},
	})
}
