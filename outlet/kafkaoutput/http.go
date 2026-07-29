// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaoutput

import (
	"net/http"
)

// SchemaHTTPHandler serves the .proto definition of the messages produced on
// the output topic. Consumers need it to decode the flows and it changes with
// the schema, like the topic name does.
func (c *Component) SchemaHTTPHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(c.d.Schema.ProtobufDefinition()))
}
