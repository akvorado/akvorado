# Configuration

Each *Akvorado* service is configured through a YAML file. You can get
the default configuration with `./akvorado SERVICE --dump --check`.
Durations can be written in seconds or using strings like `10h20m`.

It is also possible to override configuration settings using
environment variables. You need to remove any `-` from key names and
use `_` to handle nesting. Then, put `AKVORADO_SERVICE_` as a prefix
where `SERVICE` should be replaced by the service name (`inlet`,
`configure` or `console`). For example, let's consider the following
configuration file for the *inlet* service:

```yaml
http:
  listen: 127.0.0.1:8081
kafka:
  topic: test-topic
  brokers:
    - 192.0.2.1:9092
    - 192.0.2.2:9092
```

It can be translated to:

```sh
AKVORADO_INLET_HTTP_LISTEN=127.0.0.1:8081
AKVORADO_INLET_KAFKA_TOPIC=test-topic
AKVORADO_INLET_KAFKA_BROKERS=192.0.2.1:9092,192.0.2.2:9092
```

Each service is split into several functional components. Each of them
gets a section of the configuration file matching its name.

## Inlet service

The main components of the inlet services are `flow`, `kafka`, and
`core`.

### Flow

The flow component handles incoming flows. It only accepts the
`inputs` key to define the list of inputs to receive incoming flows.

Each input has a `type` and a `decoder`. For `decoder`, only `netflow`
is currently supported. As for the `type`, both `udp` and `file` are
supported.

For the UDP input, the supported keys are `listen` to set the
listening endpoint, `workers` to set the number of workers to listen
to the socket, `receive-buffer` to set the size of the kernel's
incoming buffer for each listening socket, and `queue-size` to define
the number of messages to buffer inside each worker. For example:

```yaml
flow:
  inputs:
    - type: udp
      decoder: netflow
      listen: 0.0.0.0:2055
      workers: 3
  workers: 2
```

The `file` input should only be used for testing. It supports a
`paths` key to define the files to read from. These files are injected
continuously in the pipeline. For example:

```yaml
flow:
  inputs:
    - type: file
      decoder: netflow
      paths:
       - /tmp/flow1.raw
       - /tmp/flow2.raw
  workers: 2
```

Without configuration, *Akvorado* will listen for incoming
Netflow/IPFIX flows on a random port (check the logs to know which
one).

### Kafka

Received flows are exported to a Kafka topic using the [protocol
buffers format][]. The definition file is `flow/flow-*.proto`. Each
flow is written in the [length-delimited format][].

[protocol buffers format]: https://developers.google.com/protocol-buffers
[length-delimited format]: https://cwiki.apache.org/confluence/display/GEODE/Delimiting+Protobuf+Messages

The following keys are accepted:

- `brokers` specifies the list of brokers to use to bootstrap the
  connection to the Kafka cluster
- `version` tells which minimal version of Kafka to expect
- `topic` defines the base topic name
- `flush-interval` defines the maximum flush interval to send received
  flows to Kafka
- `flush-bytes` defines the maximum number of bytes to store before
  flushing flows to Kafka
- `max-message-bytes` defines the maximum size of a message (it should
  be equal or smaller to the same setting in the broker configuration)
- `compression-codec` defines the compression codec to use to compress
  messages (`none`, `gzip`, `snappy`, `lz4` and `zstd`)
- `queue-size` defines the size of the internal queues to send
  messages to Kafka. Increasing this value will improve performance,
  at the cost of losing messages in case of problems.

The topic name is suffixed by the version of the schema. For example,
if the configured topic is `flows` and the current schema version is
1, the topic used to send received flows will be `flows-v1`.

For example:

```yaml
kafka:
  topic: test-topic
  brokers: 10.167.19.3:9092,10.167.19.4:9092,10.167.19.5:9092
  compression-codec: zstd
```

### Core

The core component queries the `geoip` and the `snmp` component to
hydrates the flows with additional information. It also classifies
exporters and interfaces into groups with a set of classification
rules.

The following configuration keys are accepted:

- `workers` key define how many workers should be spawned to process
  incoming flows
- `exporter-classifiers` is a list of classifier rules to define a group
  for exporters
- `interface-classifiers` is a list of classifier rules to define
  connectivity type, network boundary and provider for an interface
- `classifier-cache-size` defines the size of the classifier cache. As
  classifiers are pure, their result is cached in a cache. The metrics
  should tell if the cache is big enough. It should be set at least to
  twice the number of the most busy interfaces.

Classifier rules are written using [expr][].

Exporter classifiers gets the classifier IP address and its hostname.
If they can make a decision, they should invoke one of the
`Classify()` functions with the target group as an argument. Calling
this function makes the exporter part of the provided group. Evaluation
of rules stop on first match. The accessible variables and functions
are:

- `Exporter.IP` for the exporter IP address
- `Exporter.Name` for the exporter name
- `Classify()` to classify exporter to a group

Interface classifiers gets the following information and, like exporter
classifiers, should invoke one of the `Classify()` functions to make a
decision:

- `Exporter.IP` for the exporter IP address
- `Exporter.Name` for the exporter name
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

### GeoIP

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

### SNMP

Flows only include interface indexes. To associate them with an
interface name and description, SNMP is used to poll the exporter
sending each flows. A cache is maintained to avoid polling
continuously the exporters. The following keys are accepted:

- `cache-duration` tells how much time to keep data in the cache
- `cache-refresh` tells how much time to wait before updating an entry
  by polling it
- `cache-check-interval` tells how often to check if cached data is
  about to expire or need an update
- `cache-persist-file` tells where to store cached data on shutdown and
  read them back on startup
- `default-community` tells which community to use when polling exporters
- `communities` is a map from a exporter IP address to the community to
  use for a exporter, overriding the default value set above,
- `poller-retries` is the number of retries on unsuccessful SNMP requests.
- `poller-timeout` tells how much time should the poller wait for an answer.
- `workers` tell how many workers to spawn to handle SNMP polling.

As flows missing interface information are discarded, persisting the
cache is useful to quickly be able to handle incoming flows. By
default, no persistent cache is configured.

### HTTP

The builtin HTTP server serves various pages. Its configuration
supports only the `listen` key to specify the address and port to
listen. For example:

```yaml
http:
  listen: 0.0.0.0:8000
```

### Reporting

Reporting encompasses logging and metrics. Currently, as *Akvorado* is
expected to be run inside Docker, logging is done on the standard
output and is not configurable. As for metrics, they are reported by
the HTTP component on the `/api/v0/inlet/metrics` endpoint and there is
nothing to configure either.

## Configuration service

The two main components of the configuration service are `clickhouse`
and `kafka`. It also uses the [HTTP](#http) and
[reporting](#reporting) component from the inlet service and accepts
the same configuration settings.

### ClickHouse

The ClickHouse component exposes some useful HTTP endpoints to
configure a ClickHouse database. It also provisions and keep
up-to-date a ClickHouse database. The following keys should be
provided:

 - `servers` defines the list of ClickHouse servers to connect to
 - `username` is the username to use for authentication
 - `password` is the password to use for authentication
 - `database` defines the database to use to create tables
 - `orchestrator-url` defines the URL of the orchestrator to be used
   by Clickhouse (autodetection when not specified)
 - `kafka` defines the configuration for Kafka. It takes `topic`,
   `brokers` and `version`, as described in the configuration for the
   [inlet service](#kafka) but if absent, they are copied over from
   the Kafka component. It also takes `consumers` to define the number
   of consumers to use to poll Kafka (it should not exceed the number
   of partitions)

### Kafka

The Kafka component creates or updates the Kafka topic to receive
flows. It accepts the following keys:

 - `topic`, `brokers` and `version` keys are described in the
   configuration for the [inlet service](#kafka)
 - `topic-configuration` describes how the topic should be configured

The following keys are accepted for the topic configuration:

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

Currently, the configure service won't update the replication factor.
The configuration entries are kept in sync with the content of the
configuration file.

## Console service

The main components of the console service are `http` and `console`.
`http` accepts the [same configuration](#http) as for the inlet
service. The `console` has no configuration.
