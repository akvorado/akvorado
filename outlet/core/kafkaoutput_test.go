// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"net/netip"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"

	"akvorado/common/constants"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafkainput"
	"akvorado/outlet/kafkaoutput"
	"akvorado/outlet/metadata"
	"akvorado/outlet/routing"
)

// finalizingClickHouse is a minimal ClickHouse mock whose worker actually
// finalizes the flow (like the real worker), so the Protobuf message is
// populated and the worker's kafka-output path runs. The shared clickhouse.NewMock
// clears instead of finalizing, which would leave ProtobufMessage() empty.
type finalizingClickHouse struct{}

func (finalizingClickHouse) NewWorker(_ int, bf *schema.FlowMessage) clickhouse.Worker {
	return &finalizingWorker{bf: bf}
}

type finalizingWorker struct{ bf *schema.FlowMessage }

func (w *finalizingWorker) FinalizeAndSend(context.Context) clickhouse.WorkerStatus {
	w.bf.Finalize()
	return clickhouse.WorkerStatusIdle
}

func (w *finalizingWorker) Flush(context.Context) { w.bf.Clear() }

// TestCoreKafkaOutput wires an enabled Kafka output into the worker and checks the
// enriched flow is both stored to ClickHouse and produced to the kafka-output
// topic. This covers the worker's dual-encode/Send path, which is inert (and
// thus untested) whenever the output is disabled.
func TestCoreKafkaOutput(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)

	kafkaOutputConfig := kafkaoutput.DefaultConfiguration()
	kafkaOutputConfig.Enabled = true
	outputTopic := kafkaOutputConfig.Topic + "-" + sch.ProtobufMessageHash()

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, outputTopic),
		kfake.WithLogger(kafka.NewLogger(r)),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	t.Cleanup(cluster.Close)

	daemonComponent := daemon.NewMock(t)
	httpComponent := httpserver.NewMock(t, r)

	// Kafka output, enabled and pointed at the fake broker.
	kafkaOutputConfig.Brokers = cluster.ListenAddrs()
	kafkaOutputComponent, err := kafkaoutput.New(r, kafkaOutputConfig, kafkaoutput.Dependencies{
		Daemon: daemonComponent,
		HTTP:   httpComponent,
		Schema: sch,
	})
	if err != nil {
		t.Fatalf("kafkaoutput.New() error:\n%+v", err)
	}

	// Remaining core dependencies.
	metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(),
		metadata.Dependencies{Daemon: daemonComponent})
	flowComponent, err := flow.New(r, flow.DefaultConfiguration(), flow.Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("flow.New() error:\n%+v", err)
	}
	routingComponent := routing.NewMock(t, r)
	routingComponent.PopulateRIB(t)
	kafkaInputComponent, incoming := kafkainput.NewMock(t, kafkainput.DefaultConfiguration())
	clickhouseComponent := finalizingClickHouse{}

	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:      daemonComponent,
		Flow:        flowComponent,
		Metadata:    metadataComponent,
		KafkaInput:  kafkaInputComponent,
		ClickHouse:  clickhouseComponent,
		KafkaOutput: kafkaOutputComponent,
		HTTP:        httpComponent,
		Routing:     routingComponent,
		Schema:      sch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, kafkaOutputComponent)
	helpers.StartStop(t, c)

	// Inject one enriched flow.
	msg := &schema.FlowMessage{
		TimeReceived:    200,
		SamplingRate:    1000,
		ExporterAddress: helpers.AddrTo6(netip.MustParseAddr("192.0.2.142")),
		InIf:            434,
		OutIf:           677,
		SrcAddr:         netip.MustParseAddr("::ffff:67.43.156.77"),
		DstAddr:         netip.MustParseAddr("::ffff:2.125.160.216"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnBytes:   uint64(6765),
			schema.ColumnPackets: uint64(4),
			schema.ColumnEType:   uint32(0x800),
			schema.ColumnProto:   uint32(constants.ProtoTCP),
			schema.ColumnSrcPort: uint16(8534),
			schema.ColumnDstPort: uint16(80),
		},
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		t.Fatalf("gob.Encode() error:\n%+v", err)
	}
	rawFlow := &pb.RawFlow{
		TimeReceived:    uint64(time.Now().Unix()),
		Payload:         buf.Bytes(),
		SourceAddress:   msg.ExporterAddress.AsSlice(),
		Decoder:         pb.RawFlow_DECODER_GOB,
		TimestampSource: pb.RawFlow_TS_INPUT,
	}
	data, err := rawFlow.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT() error:\n%+v", err)
	}
	incoming <- data

	// The flow is forwarded to ClickHouse and, in parallel, produced to Kafka.
	expectedMetrics := map[string]string{"sent_messages_total": "1"}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	for {
		gotMetrics := r.GetMetrics("akvorado_outlet_kafkaoutput_", "sent_messages_total")
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			select {
			case <-ctx.Done():
				t.Fatalf("kafka-output sent metric (-got, +want):\n%s", diff)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}
}
