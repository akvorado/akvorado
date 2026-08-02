# Command line and endpoints

*Akvorado* uses a subcommand system. Each subcommand has its own set of options.
You can get help with `akvorado --help`. Start each service with the matching
subcommand. When started from a TTY, a service displays logs in a special
format. Without a TTY, logs are formatted as JSON.

## Common options

Each service accepts a set of common options as flags.

The `--check` option checks if the provided configuration is correct and then
stops. The `--dump` option dumps the parsed configuration with the default
values. Combine it with `--check` if you do not want the service to start.

Each service requires either a configuration file (in YAML format) or a URL to
fetch its configuration (in JSON format) as an argument.
See the [configuration section](50-configuration.md) for more information.

Only the orchestrator service should get a configuration file. The other
services should point to it.

```console
$ akvorado orchestrator /etc/akvorado/akvorado.yaml
$ akvorado inlet http://orchestrator:8080
$ akvorado outlet http://orchestrator:8080
$ akvorado console http://orchestrator:8080
$ akvorado console http://orchestrator:8080#2
```

Each service has an HTTP server that exposes a few endpoints. All services
expose these endpoints in addition to the service-specific endpoints:

- `/api/v0/metrics`: Prometheus metrics
- `/api/v0/version`: *Akvorado* version
- `/api/v0/healthcheck`: are we alive?

Each endpoint is also exposed under the service namespace. The idea is to
expose a unified API for all services under a single endpoint with an HTTP
proxy. For example, the `inlet` service also exposes its metrics under
`/api/v0/inlet/metrics` and the `outlet` service exposes its metrics under
`/api/v0/outlet/metrics`.

## Inlet service

`akvorado inlet` starts the inlet service. It receives NetFlow/IPFIX/sFlow
packets and sends them to Kafka. The inlet service does not expose any
service-specific HTTP endpoints.

## Outlet service

`akvorado outlet` starts the outlet service. It takes flows from Kafka,
parses them, adds metadata and routing information, and sends them to
ClickHouse. The HTTP component in the service exposes these endpoints:

- `/api/v0/outlet/flows`: streams the received flows. Use this for debugging
  only, as it has a performance impact.
- `/api/v0/outlet/kafka-output/schema.proto`: the `.proto` definition of the
  messages produced on the [Kafka output](50-configuration.md#kafka-output)
  topic. Only present when this output is enabled.

Consumers of the Kafka output need this definition to decode the flows. The
message name carries the same hash as the topic name, so you can check the two
agree:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/kafka-output/schema.proto
syntax = "proto3";

message FlowMessagev6VOQ4CRHFAVLBGRTLIP4T2N7CU {
 uint32 TimeReceived = 1;
 uint64 SamplingRate = 2;
 bytes ExporterAddress = 3;
 string ExporterName = 4;
[...]
}
```

A few wire conventions are not visible in the field types alone: IP addresses are
16-byte values in IPv6 form (IPv4 addresses are mapped into IPv6), `Enum8`
columns carry their numeric value, and `Array(UInt128)` elements are 16 bytes,
high 64 bits then low 64 bits, big-endian.

## Orchestrator service

`akvorado orchestrator` starts the orchestrator service. It runs as a service
and exposes an HTTP service for other components (internal and external) to
configure themselves. The Kafka topic is configured at startup and does not
need the service to be running.

These endpoints are exposed to configure other internal services:

- `/api/v0/orchestrator/configuration/inlet`
- `/api/v0/orchestrator/configuration/outlet`
- `/api/v0/orchestrator/configuration/console`

These endpoints are exposed for ClickHouse to use:

- `/api/v0/orchestrator/clickhouse/protocols.csv` contains a CSV with the mapping
  between protocol numbers and names
- `/api/v0/orchestrator/clickhouse/asns.csv` contains a CSV with the mapping
  between AS numbers and organization names

ClickHouse clusters are not currently supported, but you can configure several
servers in the configuration. Several servers are managed as if they are copies
of each other.

*Akvorado* also handles database migration during upgrades. When the protobuf
schema is updated, new Kafka tables and the associated materialized view should
be created. Older tables should be kept, especially during rolling upgrades
when some *akvorado* instances are still running an older version.

## Console service

`akvorado console` starts the console service. It provides the web interface to
explore the flows stored in ClickHouse. The pages and the filter language are
described in the [console reference](52-console.md).

## Demo exporter service

The demo exporter service simulates a NetFlow exporter, a simple SNMP agent, and
a BMP exporter.

> [!INFO]
> The demo exporter is not enabled by default. You need to run `docker compose
> --profile demo up -d`.

## Other commands

- `akvorado version` displays the version.
