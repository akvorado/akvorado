package metrics

import (
	"fmt"

	"flowexporter/reporter/logger"
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
