# Usage

*Akvorado* uses a subcommand system. Each subcommand comes with its
own set of options. It is possible to get help using `akvorado
--help`.

## Starting Akvorado

`akvorado serve` starts *Akvorado* itself, allowing it to receive and
process flows. When started from a TTY, it will display logs in a
fancy way. Without a TTY, logs are output formatted as JSON.

The `--config` options allows to provide a configuration file in YAML
format. See the [configuration section](02-configuration.md) for more
information on this file.

The `--check` option will check if the provided configuration is
correct and stops here. The `--dump` option will dump the parsed
configuration, along with the default values. It should be combined
with `--check` if you don't want *Akvorado* to start.

## Exposed HTTP endpoints

The embedded HTTP server contains the following endpoints:

- [`/api/v0/metrics`](/api/v0/metrics): Prometheus metrics
- [`/api/v0/version`](/api/v0/version): *Akvorado* version
- [`/api/v0/healthcheck`](/api/v0/healthcheck): are we alive?
- [`/api/v0/flows`](/api/v0/flows?limit=1): next available flows
- [`/api/v0/schemas.json`](/api/v0/schemas.json): versioned list of protobuf schemas used to export flows
- [`/api/v0/schema-X.proto`](/api/v0/schema-1.proto): protobuf schema used to export flows
- `/api/v0/clickhouse`: various endpoints for [ClickHouse integration](04-integration.md#clickhouse)

The [`/api/v0/flows`](/api/v0/flows?limit=1) continously printed flows
sent to Kafka (using [ndjson]()). It also accepts a `limit` argument
to stops after emitting the specified number of flows. This endpoint
should not be used for anything else other than debug: it can skips
some flows and if there are several users, flows will be dispatched
between them.

[ndjson]: http://ndjson.org/

## Other commands

`akvorado version` displays the version.
