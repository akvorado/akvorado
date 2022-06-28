// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"fmt"

	"akvorado/common/reporter/logger"
)

// promHTTPLogger is an adapter for logger.Logger to be used as promhttp.Logger
type promHTTPLogger struct {
	l logger.Logger
}

// Println outputs
func (m promHTTPLogger) Println(v ...interface{}) {
	if e := m.l.Debug(); e.Enabled() {
		e.Msg(fmt.Sprint(v...))
	}
}
