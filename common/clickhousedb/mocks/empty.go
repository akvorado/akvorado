// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package mocks contains mocks for clickhousedb package.
package mocks

import (
	_ "github.com/ClickHouse/clickhouse-go/v2/lib/driver" // for mockgen in vendor mode
	_ "go.uber.org/mock/mockgen/model"                    // for mockgen in vendor mode
)
