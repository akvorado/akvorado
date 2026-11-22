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

ClickHouse clusters are supported with the [`cluster`
setting](50-configuration.md#clickhouse-database): the orchestrator then runs
the schema migrations on the whole cluster and uses replicated tables. With more
than one shard, it also creates the matching distributed tables. Without a
cluster name, you can still configure several servers, but they are managed as
if they are copies of each other.

*Akvorado* also handles database migration during upgrades. When the protobuf
schema is updated, new Kafka tables and the associated materialized view should
be created. Older tables should be kept, especially during rolling upgrades
when some *akvorado* instances are still running an older version.

## Console service

`akvorado console` starts the console service. It provides the web interface to
explore the flows stored in ClickHouse. The pages and the filter language are
described in the [console reference](52-console.md).

It also serves this documentation on `/api/v0/console/docs/{name}`, where
`{name}` is the word part of the file name, like `configuration`. The answer is
JSON with `Accept: application/json`, and the Markdown source in every other
case. The pages of the web interface, like `/docs/configuration`, also return
the source, unless the request asks for `text/html`. A browser does, so it still
gets the web interface.

## Demo exporter service

The demo exporter service simulates a NetFlow exporter, a simple SNMP agent, and
a BMP exporter. The flows come from a small generative model embedded in the
binary. You can train your own from the flows of a real network stored in
ClickHouse.

> [!NOTE]
> The demo exporter is not enabled by default. You need to run `docker compose
> --profile demo up -d`.

### Training a flow model

`akvorado demo-exporter train train.yaml` trains a model from a ClickHouse
database. The model stays small, trains in a few minutes on a CPU, and anonymize
data:

- only flows crossing the edge of the network are used: entering on an external
  interface and leaving on a non-external one, or the opposite,
- local prefixes, on the side of the other interface, are replaced by random
  prefixes of the same length under the `anonymization` prefixes,
- remote prefixes are only kept when they are public, not more specific than a
  /24 or a /48, with a public origin AS number, not seen as a local prefix, and
  outside `exclude-asns` and `exclude-prefixes`,
- prefixes, protocols, and ports seen in fewer than `min-count` sampled flows
  are left out,
- interface names, descriptions and speeds, AS paths, communities, forwarding
  status, and your AS number are never read: only the boundary of the
  customer-side interface is kept. Exporter names are only used to select
  flows with the optional `exporters` key,
- training uses a fraction of the flows and only the shape of the hourly traffic
  is stored, not its volume.

```yaml
clickhouse:
  servers:
    - clickhouse:9000
  database: default
start: 2026-08-01T00:00:00Z
end: 2026-08-08T00:00:00Z
fraction: 0.1
exclude-asns:
  - 64496
  - 64497
exclude-prefixes: 198.51.100.0/24,2001:db8::/32
anonymization:
  ipv4-prefix: 98.0.0.0/8
  ipv6-prefix: 3fff::/20
output: custom.model
```

List the AS numbers of your network and of its group in `exclude-asns`, and your
address space in `exclude-prefixes`. The model remembers the top remote prefixes
with their origin AS numbers, the protocols, the ports, the packet sizes, and
the hourly traffic pattern.

Use `akvorado demo-exporter inspect custom.model` to review what a model
contains before sharing it, with a few generated flows (`--flows` sets how
many). Without argument, it inspects the default model.

The `model` and `train` sections tune the model size and the training loop and
can usually be left to their defaults. Exporter identities and interfaces are
not part of the model: they are reintroduced at generation time by the `mapping`
section of the flows configuration, which distributes traffic shares over a
fleet of synthetic exporters, by percentage and by remote AS number.

## Other commands

- `akvorado version` displays the version.
