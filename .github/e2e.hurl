# Test last flow widget
GET http://127.0.0.1:8080/api/v0/console/widget/flow-last
HTTP 200
[Asserts]
jsonpath "$.InIfName" matches /^Gi0\/.*/
jsonpath "$.OutIfName" matches /^Gi0\/.*/
jsonpath "$.ExporterRole" == "edge"
jsonpath "$.SamplingRate" == 100000

# Test metrics
GET http://127.0.0.1:8080/api/v0/console/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/inlet/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/outlet/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/orchestrator/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/

# Test some Prometheus queries

## Test we have the 4 services running.
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: akvorado_cmd_info{job=~"akvorado-.+"}
HTTP 200
[Asserts]
jsonpath "$.data.result" count == 4
jsonpath "$.data.result[*].metric.job" contains "akvorado-orchestrator"
jsonpath "$.data.result[*].metric.job" contains "akvorado-console"
jsonpath "$.data.result[*].metric.job" contains "akvorado-inlet"
jsonpath "$.data.result[*].metric.job" contains "akvorado-outlet"

## Test inlet has received flows from 4 exporters
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum by (exporter) (akvorado_inlet_flow_input_udp_bytes_total{job="akvorado-inlet",listener=":2055"})
HTTP 200
[Asserts]
jsonpath "$.data.result" count == 4

## Test inlet has received NetFlow flows
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum(akvorado_inlet_flow_input_udp_packets_total{job="akvorado-inlet",listener=":2055"})
HTTP 200
[Captures]
inlet_receivedflows: jsonpath "$.data.result[0].value[1]" toInt
[Asserts]
variable "inlet_receivedflows" > 10

## Test inlet has sent them to Kafka
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum(akvorado_inlet_kafka_sent_messages_total{job="akvorado-inlet"})
HTTP 200
[Captures]
inlet_sentflows: jsonpath "$.data.result[0].value[1]" toInt
[Asserts]
variable "inlet_sentflows" >= {{ inlet_receivedflows }}

## Check outlet received flows from Kafka
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: akvorado_outlet_kafka_received_bytes_total{job="akvorado-outlet"}
HTTP 200
[Captures]
outlet_receivedrawflows: jsonpath "$.data.result[0].value[1]" toInt
[Asserts]
variable "outlet_receivedrawflows" >= {{ inlet_sentflows }}

## Check outlet decoded flows
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum(akvorado_outlet_core_received_flows_total{job="akvorado-outlet"})
HTTP 200
[Captures]
outlet_receivedflows: jsonpath "$.data.result[0].value[1]" toInt
[Asserts]
variable "outlet_receivedflows" < {{ outlet_receivedrawflows }}

## Check outlet forwarded flows
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum(akvorado_outlet_core_forwarded_flows_total{job="akvorado-outlet"})
HTTP 200
[Captures]
outlet_forwardedflows: jsonpath "$.data.result[0].value[1]" toInt
[Asserts]
variable "outlet_forwardedflows" >= {{ outlet_receivedflows }}

## Check outlet forwarded flows to ClickHouse
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: sum(akvorado_outlet_clickhouse_flow_per_batch{job="akvorado-outlet", quantile="0.5"})
HTTP 200
[Asserts]
jsonpath "$.data.result[0].value[1]" toInt > 500

## Check we receive metrics from everything
GET http://127.0.0.1:8080/prometheus/api/v1/query
[Query]
query: up==0
HTTP 200
[Asserts]
jsonpath "$.data.result" count == 0

# Test some Loki queries

## Check we have logs about akvorado starting
GET http://127.0.0.1:8080/loki/loki/api/v1/query_range
[Query]
since: 1h
query: {service_name=~"akvorado.+"} |= "akvorado has started"
HTTP 200
[Asserts]
jsonpath "$.data.result" count > 0
