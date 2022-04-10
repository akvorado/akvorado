# Usage

*Akvorado* uses a subcommand system. Each subcommand comes with its
own set of options. It is possible to get help using `akvorado
--help`. Each service is started using the matchin subcommand. When
started from a TTY, a service displays logs in a fancy way. Without a
TTY, logs are output formatted as JSON.

## Common options

Each service accepts a set of common options as flags.

The `--check` option will check if the provided configuration is
correct and stops here. The `--dump` option will dump the parsed
configuration, along with the default values. It should be combined
with `--check` if you don't want the service to start.

Each service requires as an argument either a configuration file (in
YAML format) or an URL to fetch their configuration (in JSON format).
See the [configuration section](02-configuration.md) for more
information.

It is expected that only the orchestrator service gets a configuration
file and the other services should point to it.

```console
$ akvorado orchestrator /etc/akvorado/config.yaml
$ akvorado inlet http://orchestrator:8080
$ akvorado console http://orchestrator:8080
```

Each service embeds an HTTP server exposing a few endpoints. All
services expose the following endpoints in addition to the
service-specific endpoints:

- `/api/v0/metrics`: Prometheus metrics
- `/api/v0/version`: *Akvorado* version
- `/api/v0/healthcheck`: are we alive?

Each endpoint is also exposed under the service namespace. The idea is
to be able to expose an unified API for all services under a single
endpoint using an HTTP proxy. For example, the `inlet` service also
exposes its metrics under `/api/v0/inlet/metrics`.

## Inlet service

`akvorado inlet` starts the inlet service, allowing it to receive and
process flows. The following endpoints are exposed by the HTTP
component embedded into the service:

- `/api/v0/inlet/flows`: stream the received flows
- `/api/v0/inlet/schemas.json`: versioned list of protobuf schemas used to export flows
- `/api/v0/inlet/schemas-X.proto`: protobuf schema for the provided version

## Orchestrator service

`akvorado orchestrator` starts the orchestrator service. It runs as a
service as it exposes an HTTP service for other components (internal
and external) to configure themselves. The Kafka topic is configured
at start and does not need the service to be running.

The following endpoints are exposed to configure other internal
services:

- `/api/v0/orchestrator/broker/configuration/inlet`
- `/api/v0/orchestrator/broker/configuration/console`

The following endpoints are exposed for use by ClickHouse:

- `/api/v0/orchestrator/clickhouse/init.sh` contains the schemas in the form of a
  script to execute during initialization to get them installed at the
  proper location
- `/api/v0/orchestrator/clickhouse/protocols.csv` contains a CSV with the mapping
  between protocol numbers and names
- `/api/v0/orchestrator/clickhouse/asns.csv` contains a CSV with the mapping
  between AS numbers and organization names

ClickHouse clusters are currently not supported, despite being able to
configure several servers in the configuration. Several servers are in
fact managed like they are a copy of one another.

*Akvorado* also handles database migration during upgrades. When the
protobuf schema is updated, new Kafka tables should be created, as
well as the associated materialized view. Older tables should be kept
around, notably when upgrades can be rolling (some *akvorado*
instances are still running an older version).

## Console service

`akvorado console` starts the console service. Currently, only this
documentation is accessible through this service.

## Other commands

`akvorado version` displays the version.
