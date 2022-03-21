# Usage

*Akvorado* uses a subcommand system. Each subcommand comes with its
own set of options. It is possible to get help using `akvorado
--help`.

## version

`akvorado version` displays the version.

## serve

`akvorado serve` starts *Akvorado* itself, allowing it to receive and
process flows. When started from a TTY, it will display logs in a
fancy way. Without a TTY, logs are output using JSON.

The `--config` options allows to provide a configuration file in YAML
format. See the [configuration section](configuration.md) for more
information on this file.

The `--check` option will check if the provided configuration is
correct and stops here. The `--dump` option will dump the parsed
configuration, along with the default values. It should be combined
with `--check` if you don't want *Akvorado* to start.

### Exposed HTTP endpoints

The embedded HTTP server contains the endpoints listed on the [home
page](index.md). The [`/api/v0/flows`](/api/v0/flows?limit=1)
continously printed flows sent to Kafka (using [ndjson]()). It also
accepts a `limit` argument to stops after emitting the specified
number of flows. This endpoint should not be used for anything else
other than debug: it can skips some flows and if there are several
users, flows will be dispatched between them.

[ndjson]: http://ndjson.org/
