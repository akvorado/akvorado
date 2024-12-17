// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse_test

import (
	"fmt"
	"sync"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
)

func TestMock(t *testing.T) {
	sch := schema.NewMock(t)
	bf := sch.NewFlowMessage()

	var messages []*schema.FlowMessage
	var messagesMutex sync.Mutex
	ch := clickhouse.NewMock(t, func(msg *schema.FlowMessage) {
		messagesMutex.Lock()
		defer messagesMutex.Unlock()
		messages = append(messages, msg)
	})
	helpers.StartStop(t, ch)

	expected := []*schema.FlowMessage{}
	w := ch.NewWorker(1, bf)
	for i := range 20 {
		i = i + 1 // 1 to 20
		bf.TimeReceived = uint32(100 + i)
		bf.SrcAS = uint32(65400 + i)
		bf.DstAS = uint32(65500 + i)
		bf.AppendString(schema.ColumnExporterName, fmt.Sprintf("exporter-%d", i))
		expected = append(expected, &schema.FlowMessage{
			TimeReceived: bf.TimeReceived,
			SrcAS:        bf.SrcAS,
			DstAS:        bf.DstAS,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnExporterName: fmt.Sprintf("exporter-%d", i),
			},
		})
		w.FinalizeAndSend(t.Context())

		// Check if we have anything inserted in the table
		messagesMutex.Lock()
		if diff := helpers.Diff(messages, expected); diff != "" {
			t.Fatalf("Mock(), iteration %d, (-got, +want):\n%s", i, diff)
		}
		messagesMutex.Unlock()
	}
}
