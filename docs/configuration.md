# Configuration

*Akvorado* can be configured through a YAML file. Each aspect is
configured through a different section:

- `reporting`: [Log and metric reporting](#reporting)
- `http`: [Builtin HTTP server](#http)
- `web`: [Web interface](#web)
- `flow`: [Flow ingestion](#flow)
- `snmp`: [SNMP poller](#snmp)
- `geoip`: [GeoIP database](#geoip)
- `kafka`: [Kafka broker](#kafka)
- `clickhouse`: [ClickHouse helper](#clickhouse)
- `core`: [Core](#core)

You can get the default configuration with `./akvorado --dump
--check`. Durations can be written in seconds or using strings like
`10h20m`.

It is also possible to override configuration settings using
environment variables. You need to remove any `-` from key names and
use `_` to handle nesting. Then, put `AKVORADO_` as a prefix. For
example, let's consider the following configuration file:

```yaml
kafka:
  topic: test-topic
  topic-configuration:
    num-partitions: 1
  brokers:
    - 192.0.2.1:9092
    - 192.0.2.2:9092
```

It can be translated to:

```sh
AKVORADO_KAFKA_TOPIC=test-topic
AKVORADO_KAFKA_TOPICCONFIGURATION_NUMPARTITIONS=1
AKVORADO_KAFKA_BROKERS=192.0.2.1:9092,192.0.2.2:9092
```

## Reporting

Reporting encompasses logging and metrics. Currently, as *Akvorado* is
expected to be run inside Docker, logging is done on the standard
output and is not configurable. As for metrics, they are reported by
the HTTP component on the `/api/v0/metrics` endpoint and there is
nothing to configure either.

## HTTP

The builtin HTTP server serves various pages. Its configuration
supports only the `listen` key to specify the address and port to
listen. For example:

```yaml
http:
  listen: 0.0.0.0:8000
```

## Web

The web interface presents the landing page of *Akvorado*. It also
embeds the documentation. It accepts only the following key:

- `grafanaurl` to specify the URL to Grafana and exposes it as
  [`/grafana`](/grafana).

## Flow

The flow component handles flow ingestion. It supports the following
configuration keys:

- `listen` to specify the IP and UDP port to listen for new flows
- `workers` to specify the number of workers to spawn to handle
  incoming flows
- `queue-size` to specify the number of flows to queue when pushing
  them to the core component

For example:

```yaml
flow:
  listen: 0.0.0.0:2055
  workers: 2
```

## SNMP

Flows only include interface indexes. To associate them with an
interface name and description, SNMP is used to poll the sampler
sending each flows. A cache is maintained to avoid polling
continuously the samplers. The following keys are accepted:

- `cache-duration` tells how much time to keep data in the cache before
  polling again
- `cache-refresh` tells how much time to poll existing data before they
  expire
- `cache-refresh-interval` tells how often to check if cached data is
  about to expire
- `cache-persist-file` tells where to store cached data on shutdown and
  read them back on startup
- `default-community` tells which community to use when polling samplers
- `communities` is a map from a sampler IP address to the community to
  use for a sampler, overriding the default value set above,
- `workers` tell how many workers to spawn to handle SNMP polling.

As flows missing interface information are discarded, persisting the
cache is useful to quickly be able to handle incoming flows. By
default, no persistent cache is configured.

## GeoIP

The GeoIP component adds source and destination country, as well as
the AS number of the source and destination IP if they are not present
in the received flows. It needs two databases using the [MaxMind DB
file format][], one for AS numbers, one for countries. If no database
is provided, the component is inactive. It accepts the following keys:

- `asn-database` tells the path to the ASN database
- `country-database` tells the path to the country database

[MaxMind DB file format]: https://maxmind.github.io/MaxMind-DB/

If the files are updated while *Akvorado* is running, they are
automatically refreshed.

## Kafka

Received flows are exported to a Kafka topic using the [protocol
buffers format][]. The definition file is `flow/flow-*.proto`. Each
flow is written in the [length-delimited format][].

[protocol buffers format]: https://developers.google.com/protocol-buffers
[length-delimited format]: https://cwiki.apache.org/confluence/display/GEODE/Delimiting+Protobuf+Messages

The following keys are accepted:

- `topic` tells which topic to use to write messages
- `topic-configuration` contains the topic configuration
- `brokers` specifies the list of brokers to use to bootstrap the
  connection to the Kafka cluster
- `version` tells which minimal version of Kafka to expect
- `usetls` tells if we should use TLS to connection (authentication is not supported)
- `flush-interval` defines the maximum flush interval to send received
  flows to Kafka
- `flush-bytes` defines the maximum number of bytes to store before
  flushing flows to Kafka
- `max-message-bytes` defines the maximum size of a message (it should
  be equal or smaller to the same setting in the broker configuration)
- `compression-codec` defines the compression codec to use to compress
  messages (`none`, `gzip`, `snappy`, `lz4` and `zstd`)

The topic name is suffixed by the version of the schema. For example,
if the configured topic is `flows` and the current schema version is
0, the topic used to send received flows will be `flows-v0`.

If no topic configuration is provided, the topic should already exist
in Kafka. If a configuration is provided, the topic is created if it
does not exist or updated if it does. Currently, updating the number
of partitions or the replication factor is not possible. The following
keys are accepted for the topic configuration:

- `num-partitions` for the number of partitions
- `replication-factor` for the replication factor
- `config-entries` is a mapping from configuration names to their values

For example:

```yaml
kafka:
  topic: test-topic
  topic-configuration:
    num-partitions: 1
    replication-factor: 1
    config-entries:
      segment.bytes: 1073741824
      retention.ms: 86400000
      cleanup.policy: delete
```

## ClickHouse

The ClickHouse component exposes some useful HTTP endpoints to
configure a ClickHouse database. Optionally, it will also provision
and keep up-to-date a ClickHouse database. In this case, the following
keys should be provided:

 - `servers` defines the list of ClickHouse servers to connect to
 - `username` is the username to use for authentication
 - `password` is the password to use for authentication
 - `database` defines the database to use to create tables
 - `akvorado-url` defines the URL of Akvorado to be used by Clickhouse (autodetection when not specified)

## Core

The core orchestrates the remaining components. It receives the flows
from the flow component, add some information using the GeoIP
databases and the SNMP poller, and push the resulting flow to Kafka.

The following keys are accepted:

- `workers` key define how many workers should be spawned to process
  incoming flows
- `sampler-classifiers` is a list of classifier rules to define a group
  for samplers
- `interface-classifiers` is a list of classifier rules to define
  connectivity type, network boundary and provider for an interface
- `classifier-cache-size` defines the size of the classifier cache. As
  classifiers are pure, their result is cached in a cache. The metrics
  should tell if the cache is big enough. It should be set at least to
  twice the number of the most busy interfaces.

Classifier rules are written using [expr][].

Sampler classifiers gets the classifier IP address and its hostname.
If they can make a decision, they should invoke one of the
`Classify()` functions with the target group as an argument. Calling
this function makes the sampler part of the provided group. Evaluation
of rules stop on first match. The accessible variables and functions
are:

- `Sampler.IP` for the sampler IP address
- `Sampler.Name` for the sampler name
- `Classify()` to classify sampler to a group

Interface classifiers gets the following information and, like sampler
classifiers, should invoke one of the `Classify()` functions to make a
decision:

- `Sampler.IP` for the sampler IP address
- `Sampler.Name` for the sampler name
- `Interface.Name` for the interface name
- `Interface.Description` for the interface description
- `Interface.Speed` for the interface speed
- `ClassifyConnectivity()` to classify for a connectivity type (transit, PNI, PPNI, IX, customer, core, ...)
- `ClassifyProvider()` to classify for a provider (Cogent, Telia, ...)
- `ClassifyExternal()` to classify the interface as external
- `ClassifyInternal()` to classify the interface as internal

Once an interface is classified for a given criteria, it cannot be
changed by later rule. Once an interface is classified for all
criteria, remaining rules are skipped. Connectivity and provider are somewhat normalized (down case)

Each `Classify()` function, with the exception of `ClassifyExternal()`
and `ClassifyInternal()` have a variant ending with `Regex` which
takes a string and a regex before the original string and do a regex
match. The original string is expanded using the matching parts of the
regex. The syntax is the one [from Go][].

Here is an example:

```
Interface.Description startsWith "Transit:" &&
ClassifyConnectivity("transit") &&
ClassifyExternal() &&
ClassifyProviderRegex(Interface.Description, "^Transit: ([^ ]+)", "$1")
```

[expr]: https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md
[from Go]: https://pkg.go.dev/regexp#Regexp.Expand
