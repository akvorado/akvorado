# Configuration

*Akvorado* can be configured through a YAML file. Each aspect is
configured through a different section:

- `reporting`: [Log and metric reporting](#reporting)
- `http`: [Builtin HTTP server](#http)
- `flow`: [Flow ingestion](#flow)
- `snmp`: [SNMP poller](#snmp)
- `geoip`: [GeoIP database](#geoip)
- `kafka`: [Kafka broker](#kafka)
- `core`: [Core](#core)

You can get the default configuration with `./akvorado --dump --check`.

Durations can be written in seconds or using strings like `10h20m`.

## Reporting

Reporting encompasses logging and metrics. Currently, as *Akvorado* is
expected to be run inside Docker, logging is done on the standard
output and is not configurable. As for metrics, they are reported by
the HTTP component on the `/metrics` endpoint and there is nothing to
configure either.

## HTTP

The builtin HTTP server serves various pages. Its configuration
supports only the `listen` key to specify the address and port to
listen. For example:

```yaml
http:
  listen: 0.0.0.0:8000
```

## Flow

The flow component handles flow ingestion. It supports the following
configuration keys:

- `listen` to specify the IP and UDP port to listen for new flows
- `workers` to specify the number of workers to spawn to handle
  incoming flows
- `bufferlength` to specify the number of flows to buffer when pushing
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

- `cacheduration` tells how much time to keep data in the cache before
  polling again
- `cacherefresh` tells how much time to poll existing data before they
  expire
- `cacherefreshinterval` tells how often to check if cached data is
  about to expire
- `cachepersistfile` tells where to store cached data on shutdown and
  read them back on startup
- `defaultcommunity` tells which community to use when polling samplers
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

- `asndatabase` tells the path to the ASN database
- `countrydatabase` tells the path to the country database

[MaxMind DB file format]: https://maxmind.github.io/MaxMind-DB/

If the files are updated while *Akvorado* is running, they are
automatically refreshed.

## Kafka

Received flows are exported to a Kafka topic using the [protocol
buffers format][]. The definition file is `flow/flow.proto`. It is
also available through the [`/flow.proto`](/flow.proto) HTTP endpoint.
Each flow is written in the [length-delimited format][].

[protocol buffers format]: https://developers.google.com/protocol-buffers
[length-delimited format]: https://cwiki.apache.org/confluence/display/GEODE/Delimiting+Protobuf+Messages

The following keys are accepted:

- `topic` tells which topic to use to write messages
- `autocreatetopic` tells if we can automatically create the topic if
  it does not exist
- `brokers` specifies the list of brokers to use to bootstrap the
  connection to the Kafka cluster
- `version` tells which minimal version of Kafka to expect
- `usetls` tells if we should use TLS to connection (authentication is not supported)
- `flushinterval` defines the maximum flush interval to send received
  flows to Kafka
- `flushbytes` defines the maximum number of bytes to store before
  flushing flows to Kafka
- `maxmessagebytes` defines the maximum size of a message (it should
  be equal or smaller to the same setting in the broker configuration)
- `compressioncodec` defines the compression codec to use to compress
  messages (`none`, `gzip`, `snappy`, `lz4` and `zstd`)

## Core

The core orchestrates the remaining components. It receives the flows
from the flow component, add some information using the GeoIP
databases and the SNMP poller, and push the resulting flow to Kafka.

It only accepts the `workers` key to define how many workers should be
spawn.
