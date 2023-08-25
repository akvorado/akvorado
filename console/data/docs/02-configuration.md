# Configuration

The orchestrator service is configured through YAML files (shipped in the
`config/` directory) and includes the configuration of the other services. Other
services are expected to query the orchestrator through HTTP on start to
retrieve their configuration.

The default configuration can be obtained with `docker-compose exec
akvorado-orchestrator akvorado orchestrator --dump --check /dev/null`. Note that
some sections are generated from the configuration of another section. Notably,
all Kafka configuration comes from upper-level `kafka` key. Durations must be
written using strings like `10h20m` or `5s`. Valid time units are `ms`, `s`,
`m`, and `h`.

It is also possible to override configuration settings using
environment variables. You need to remove any `-` from key names and
use `_` to handle nesting. Then, put `AKVORADO_CFG_ORCHESTRATOR_` as a
prefix. For example, let's consider the following configuration file:

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
AKVORADO_CFG_ORCHESTRATOR_HTTP_LISTEN=127.0.0.1:8081
AKVORADO_CFG_ORCHESTRATOR_KAFKA_TOPIC=test-topic
AKVORADO_CFG_ORCHESTRATOR_KAFKA_BROKERS=192.0.2.1:9092,192.0.2.2:9092
```

The orchestrator service has its own configuration, as well as the
configuration for the other services under the key matching the
service name (`inlet` and `console`). For each service, it is possible
to provide a list of configuration. A service can query the
configuration it wants by appending an index to the configuration URL.
If the index does not match a provided configuration, the first
configuration is provided.

Each service is split into several functional components. Each of them
gets a section of the configuration file matching its name.

## Inlet service

This service is configured under the `inlet` key. The main components
of the inlet services are `flow`, `kafka`, and `core`.

### Flow

The flow component handles incoming flows. It accepts the `inputs` key
to define the list of inputs to receive incoming flows and the
`rate-limit` key to have an hard-limit on the number of flows/second
accepted per exporter. When set, the provided rate limit will be
enforced for each exporter and the sampling rate of the surviving
flows will be adapted.

Each input has a `type` and a `decoder`. For `decoder`, both
`netflow` or `sflow` are supported. As for the `type`, both `udp`
and `file` are supported.

For the UDP input, the supported keys are `listen` to set the listening
endpoint, `workers` to set the number of workers to listen to the socket,
`receive-buffer` to set the size of the kernel's incoming buffer for each
listening socket, and `queue-size` to define the number of messages to buffer
inside each worker. With `use-src-addr-for-exporter-addr` set to true, the
source ip of the received flow packet is used as exporter address.

For example:

```yaml
flow:
  inputs:
    - type: udp
      decoder: netflow
      listen: :2055
      workers: 3
      use-src-addr-for-exporter-addr: true
    - type: udp
      decoder: sflow
      listen: :6343
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
    - type: file
      decoder: sflow
      paths:
       - /tmp/flow1.raw
       - /tmp/flow2.raw
  workers: 2
```

Without configuration, *Akvorado* will listen for incoming
Netflow/IPFIX and sFlow flows on a random port (check the logs to know
which one).

### Routing

The routing component optionally fetches source and destination AS numbers, as
well as the AS paths and communities. Not all exporters need to provide this
information. Currently, the default provider is BMP. *Akvorado* will try
to select the best route using the next hop advertised in the flow and fallback
to any next hop if not found.

The component accepts only a `provider` key, which defines the provider
configuration. Inside the provider configuration, the provider type is defined
by the `type` key (`bmp` and `bioris` are currently supported). The remaining
keys are specific to the provider.

#### BMP provider

For the BMP provider, the following keys are accepted:

- `listen` specifies the IP address and port to listen for incoming connections
  (default port is 10179)
- `rds` specifies a list of route distinguisher to accept (0 is meant
  to accept routes without an associated route distinguisher)
- `collect-asns` tells if origin AS numbers should be collected
- `collect-aspaths` tells if AS paths should be collected
- `collect-communities` tells if communities should be collected (both
  regular communities and large communities; extended communities are
  not supported)
- `keep` tells how much time the routes sent from a terminated BMP
  connection should be kept

If you are not interested in AS paths and communities, disabling them
will decrease the memory usage of *Akvorado*, as well as the disk
space used in ClickHouse.

*Akvorado* supports receiving the AdjRIB-in, with or without
filtering. It may also work with a LocRIB.

For example:

```yaml
routing:
  provider:
    type: bmp
    listen: 0.0.0.0:10179
    collect-asns: true
    collect-aspaths: true
    collect-communities: false
```

#### BioRIS provider

As alternative to the internal BMP, an connection to an existing [bio-rd
RIS](https://github.com/bio-routing/bio-rd/tree/master/cmd/ris) instance may be
used. It accepts the following keys:

- `ris-instances` is a list of instances
- `timeout` tells how much time to wait to get an answer from a RIS instance
- `refresh` tells how much time to wait between two refresh of the list of routers

Each instance accepts the following keys:

- `grpc-addr` is the address and port of a RIS instance
- `grpc-secure` tells if a connection should be set using TLS
- `vrf` (as a string) or `vrf-id` (as an ID) tell which VRF we should look up

This is configured as follows:

```yaml
routing:
  provider:
    type: bioris
    risinstances:
      - grpcaddr: 192.0.2.15:4321
        grpcsecure: true
        vrf: 0:0
```

BioRIS tries to query the RIB of the router that sent the flow. If this router's
RIB is not available in all known RIS instances, an other router is implictly
used as fallback. After the router id is determined, BioRIS queries one of the
RIS instances known holding the RIB.

BioRIS currently supports setting prefix, AS, AS Path and communities for the
given flow.

### Kafka

Received flows are exported to a Kafka topic using the [protocol buffers
format][]. Each flow is written in the [length-delimited format][].

[protocol buffers format]: https://developers.google.com/protocol-buffers
[length-delimited format]: https://cwiki.apache.org/confluence/display/GEODE/Delimiting+Protobuf+Messages

The following keys are accepted:

- `topic`, `brokers`, `tls`, and `version` keys are described in the
  configuration for the [orchestrator service](#kafka-1) (the values of these
  keys come from the orchestrator configuration)
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

The topic name is suffixed by a hash of the schema.

### Core

The core component queries the `geoip` and the `metadata` component to
enriches the flows with additional information. It also classifies
exporters and interfaces into groups with a set of classification
rules.

The following configuration keys are accepted:

- `workers` key define how many workers should be spawned to process
  incoming flows
- `exporter-classifiers` is a list of classifier rules to define a group
  for exporters
- `interface-classifiers` is a list of classifier rules to define
  connectivity type, network boundary and provider for an interface
- `classifier-cache-duration` defines how long to keep the result of a previous
  classification in memory to reduce CPU usage.
- `default-sampling-rate` defines the default sampling rate to use
  when the information is missing. If not defined, flows without a
  sampling rate will be rejected. Use this option only if your
  hardware is unable to advertise a sampling rate. This can either be
  a single value or a map from subnets to sampling rates.
- `override-sampling-rate` defines the sampling rate instead of the
  one received in the flows. This is useful if a device lie about its
  sampling rate. This is a map from subnets to sampling rates (but it
  would also accept a single value).
- `asn-providers` defines the source list for AS numbers. The
  available sources are `flow`, `flow-except-private` (use information
  from flow except if the ASN is private), `geoip`, `routing`, and
  `routing-except-private`. The default value is `flow`, `routing`, and
  `geoip`.
- `net-providers` defines the sources for prefix lengths and nexthop. `flow` uses the value
  provided by the flow message (if any), while `routing` looks it up using the BMP
  component. If multiple sources are provided, the value of the first source
  providing a non-default route is taken. The default value is `flow` and `routing`.

Classifier rules are written using [Expr][].

Exporter classifiers gets the classifier IP address and its hostname.
If they can make a decision, they should invoke one of the
`Classify()` functions with the target element as an argument. Once
classification is done for an element, it cannot be changed by a
subsequent rule. All strings are normalized (down case, special chars
removed).

- `Exporter.IP` for the exporter IP address
- `Exporter.Name` for the exporter name
- `ClassifyGroup()` to classify the exporter to a group
- `ClassifyRole()` to classify the exporter for a role (`edge`, `core`)
- `ClassifySite()` to classify the exporter to a site (`paris`, `berlin`, `newyork`)
- `ClassifyRegion()` to classify the exporter to a region (`france`, `italy`, `caraibes`)
- `ClassifyTenant()` to classify the exporter to a tenant (`team-a`, `team-b`)
- `Reject()` to reject the flow
- `Format()` to format a string: `Format("name: %s", Exporter.Name)`

As a compatibility `Classify()` is an alias for `ClassifyGroup()`.
Here is an example, assuming routers are named
`th2-ncs55a1-1.example.fr` or `milan-ncs5k8-2.example.it`:

```yaml
exporter-classifiers:
  - ClassifySiteRegex(Exporter.Name, "^([^-]+)-", "$1")
  - Exporter.Name endsWith ".it" && ClassifyRegion("italy")
  - Exporter.Name matches "^(washington|newyork).*" && ClassifyRegion("usa")
  - Exporter.Name endsWith ".fr" && ClassifyRegion("france")
```

Interface classifiers gets the following information and, like exporter
classifiers, should invoke one of the `Classify()` functions to make a
decision:

- `Exporter.IP` for the exporter IP address
- `Exporter.Name` for the exporter name
- `Interface.Index` for the interface index
- `Interface.Name` for the interface name
- `Interface.Description` for the interface description
- `Interface.Speed` for the interface speed
- `Interface.VLAN` for VLAN number (you need to enable `SrcVlan` and `DstVlan` in schema)
- `ClassifyConnectivity()` to classify for a connectivity type (transit, PNI, PPNI, IX, customer, core, ...)
- `ClassifyProvider()` to classify for a provider (Cogent, Telia, ...)
- `ClassifyExternal()` to classify the interface as external
- `ClassifyInternal()` to classify the interface as internal
- `SetName()` to change the interface name
- `SetDescription()` to change the interface description
- `Reject()` to reject the flow
- `Format()` to format a string: `Format("name: %s", Interface.Name)`

Once an interface is classified for a given criteria, it cannot be
changed by later rule. Once an interface is classified for all
criteria, remaining rules are skipped. Connectivity and provider are
normalized (down case, special chars removed).

Each `Classify()` function, with the exception of `ClassifyExternal()`
and `ClassifyInternal()` have a variant ending with `Regex` which
takes a string and a regex before the original string and do a regex
match. The original string is expanded using the matching parts of the
regex. The syntax is the one [from Go][]. If you want to use Perl
character classes, such as `\d` or `\w`, you need to escape the
backslash character: `\\d` and `\\w`. To test your regex, you can use
a site like [regular expressions 101][]. Be sure to use the "Golang"
flavor. You can use the substition function. In this case, append `.*`
to your regex to get the [expected result][] (you can keep it in the
final regex if you prefer).

[regular expressions 101]: https://regex101.com/
[expected result]: https://regex101.com/r/eg6drf/1

Here is an example, assuming interface descriptions for external
facing interfaces look like `Transit: Cogent 1-3834938493` or `PNI:
Netflix (WL6-1190)`.

```yaml
interface-classifiers:
  - |
    ClassifyConnectivityRegex(Interface.Description, "^(?i)(transit|pni|ppni|ix):? ", "$1") &&
    ClassifyProviderRegex(Interface.Description, "^[^ ]+? ([^ ]+)", "$1") &&
    ClassifyExternal()
  - ClassifyInternal()
```

[expr]: https://expr.medv.io/docs/Language-Definition
[from Go]: https://github.com/google/re2/wiki/Syntax

### GeoIP

The GeoIP component adds source and destination country, as well as
the AS number of the source and destination IP if they are not present
in the received flows. It needs two databases using the [MaxMind DB
file format][], one for AS numbers, one for countries. If no database
is provided, the component is inactive. It accepts the following keys:

- `asn-database` tells the path to the ASN database
- `geo-database` tells the path to the geo database (country or city)
- `optional` makes the presence of the databases optional on start
  (when not present on start, the component is just disabled)

[MaxMind DB file format]: https://maxmind.github.io/MaxMind-DB/

If the files are updated while *Akvorado* is running, they are
automatically refreshed.

### Metadata

Flows only include interface indexes. To associate them with an interface name
and description, metadata are polled. A cache is maintained. There are several
providers available to poll metadata. The following keys are accepted:

- `cache-duration` tells how much time to keep data in the cache
- `cache-refresh` tells how much time to wait before updating an entry
  by polling it
- `cache-check-interval` tells how often to check if cached data is
  about to expire or need an update
- `cache-persist-file` tells where to store cached data on shutdown and
  read them back on startup
- `workers` tell how many workers to spawn to fetch metadata.
- `max-batch-requests` define how many requests can be batched together
- `provider` defines the provider configuration

As flows missing interface information are discarded, persisting the
cache is useful to quickly be able to handle incoming flows. By
default, no persistent cache is configured.

The `provider` key contains the configuration of the provider. The provider type
is defined by the `type` key. The `snmp` provider accepts the following
configuration keys:

- `communities` is a map from exporter subnets to the SNMPv2 communities. Use
  `::/0` to set the default value. Alternatively, it also accepts a string to
  use for all exporters.
- `security-parameters` is a map from exporter subnets to the SNMPv3 USM
  security parameters. Like for `communities`, `::/0` can be used to the set the
  default value. The security paramaters accepts the following keys:
  `user-name`, `authentication-protocol` (can be omitted, otherwise `MD5`,
  `SHA`, `SHA224`, `SHA256`, `SHA384`, and `SHA512` are accepted),
  `authentication-passphrase` (if the previous value was set),
  `privacy-protocol` (can be omitted, otherwise `DES`, `AES`, `AES192`,
  `AES256`, `AES192C`, and `AES256C` are accepted, the later being
  Cisco-variant), `privacy-passphrase` (if the previous value was set), and
  `context-name`.
- `ports` is a map from exporter subnets to the SNMP port to use to poll
  exporters in the provided subnet.
- `agents` is a map from exporter IPs to agent IPs. When there is no match, the
  exporter IP is used. Other options are still using the exporter IP as a key,
  not the agent IP.
- `poller-retries` is the number of retries on unsuccessful SNMP requests.
- `poller-timeout` tells how much time should the poller wait for an answer.

For example:

```yaml
metadata:
  workers: 10
  provider:
    type: snmp
    communities:
      ::/0: private
```

*Akvorado* will use SNMPv3 if there is a match for the
`security-parameters` configuration option. Otherwise, it will use
SNMPv2.

The `gnmi` provider accepts the following keys:



The `static` provider accepts an `exporters` key which maps exporter subnets to
an exporter configuration. An exporter configuration is map:

- `name` is the name of the exporter
- `default` is the default interface when no match is found
- `ifindexes` is a map from interface indexes to interface

An interface is a `name`, a `description` and a `speed`.

For example:

```yaml
metadata
  provider:
    type: static
    exporters:
      2001:db8:1::1:
        name: exporter1
        default:
          name: unknown
          description: Unknown interface
          speed: 100
        ifindexes:
          10:
            name: Gi0/0/10
            description: PNI Netflix
            speed: 1000
          11:
            name: Gi0/0/15
            description: PNI Google
            speed: 1000
```

### HTTP

The builtin HTTP server serves various pages. Its configuration
supports the following keys:

- `listen` defines the address and port to listen to.
- `profiler` enables [Go profiler HTTP
  interface](https://pkg.go.dev/net/http/pprof). Check the [troubleshooting
  section](05-troubleshooting.html#profiling) for details. It is enabled by
  default.
- `cache` defines the cache backend to use for some HTTP requests. It accepts a
  `type` key which can be either `memory` (the default value) or `redis`. When
  using the Redis backend, the following additional keys are also accepted:
  `protocol` (`tcp` or `unix`), `server` (host and port), `username`,
  `password`, and `db` (an integer to specify which database to use).

```yaml
http:
  listen: :8000
  cache:
    type: redis
    username: akvorado
    password: akvorado
```

Note that the cache backend is currently only useful with the console. You need
to define the cache in the `http` key of the `console` section for it to be
useful (not in the `inlet` section).

### Reporting

Reporting encompasses logging and metrics. Currently, as *Akvorado* is
expected to be run inside Docker, logging is done on the standard
output and is not configurable. As for metrics, they are reported by
the HTTP component on the `/api/v0/inlet/metrics` endpoint and there is
nothing to configure either.

## Orchestrator service

The two main components of the orchestrator service are `clickhouse` and
`kafka`. It also uses the [HTTP](#http), and [reporting](#reporting) from the
inlet service and accepts the same configuration settings.

### Schema

It is possible to alter the data schema used by *Akvorado* by adding and
removing columns. For example, to add the `SrcVlan` and `DstVlan` columns while
removing the `SrcCountry` and `DstCountry`, one can use:

```yaml
schema:
  materialize:
    - SrcNetPrefix
    - DstNetPrefix
  disabled:
    - SrcCountry
    - DstCountry
  enabled:
    - SrcVlan
    - DstVlan
```

With `materialize`, you can control if an dimension computed from other
dimensions (e.g. `SrcNetPrefix` and `DstNetPrefix`) is computed at query time
(the default) or materialized at ingest time. This reduces the query time, but
increases the storage needs.

You can get the list of columns you can enable or disable with `akvorado
version`. Disabling a column won't delete existing data.

It is also possible to make make some columns available on the main table only
or on all tables with `main-table-only` and `not-main-table-only`. For example:

```yaml
schema:
  enabled:
    - SrcMAC
    - DstMAC
  main-table-only:
    - SrcMAC
    - DstMAC
  not-main-table-only:
    - SrcAddr
    - DstAddr
```

For ICMP, you get `ICMPv4Type`, `ICMPv4Code`, `ICMPv6Type`, `ICMPv6Code`,
`ICMPv4`, and `ICMPv6`. The two latest one are displayed as a string in the
console (like `echo-reply` or `frag-needed`).

### Kafka

The Kafka component creates or updates the Kafka topic to receive
flows. It accepts the following keys:

- `brokers` specifies the list of brokers to use to bootstrap the
  connection to the Kafka cluster
- `tls` defines the TLS configuration to connect to the cluster
- `version` tells which minimal version of Kafka to expect
- `topic` defines the base topic name
- `topic-configuration` describes how the topic should be configured

The following keys are accepted for the TLS configuration:

- `enable` should be set to `true` to enable TLS.
- `verify` can be set to `false` to skip checking server certificate (not recommended).
- `ca-file` gives the location of the file containing the CA certificate in PEM
  format to check the server certificate. If not provided, the system
  certificates are used instead.
- `cert-file` and `key-file` defines the location of the client certificate pair
  in PEM format to authenticate to the broker. If the first one is empty, no
  client certificate is used. If the second one is empty, the key is expected to
  be in the certificate file.
- `sasl-username` and `sasl-password` enables SASL authentication with the
  provided user and password.
- `sasl-algorithm` tells which SASL mechanism to use for authentication. This
  can be `none`, `plain`, `scram-sha256`, or `scram-sha512`. This should not be
  set to none when SASL is used.

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

Another useful setting is `retention.bytes` to limit the size of a
partition in bytes too (divide it by the number of partitions to have
a limit for the topic).

Currently, the orchestrator service won't update the replication
factor. The configuration entries are kept in sync with the content of
the configuration file.

### ClickHouse

The ClickHouse component exposes some useful HTTP endpoints to
configure a ClickHouse database. It also provisions and keep
up-to-date a ClickHouse database. The following keys should be
provided:

- `servers` defines the list of ClickHouse servers to connect to
- `username` is the username to use for authentication
- `password` is the password to use for authentication
- `database` defines the database to use to create tables
- `kafka` defines the configuration for the Kafka consumer. The accepted keys are:
  - `consumers` defines the number of consumers to use to consume messages from
    the Kafka topic. It is silently bound by the maximum number of threads
    ClickHouse will use (by default, the number of CPUs). It should also be less
    than the number of partitions: the additional consumers will stay idle.
  - `engine-settings` defines a list of additional settings for the Kafka engine
    in ClickHouse. Check [ClickHouse documentation][] for possible values. You
    can notably tune `kafka_max_block_size`, `kafka_poll_timeout_ms`,
    `kafka_poll_max_batch_size`, and `kafka_flush_interval_ms`.
- `resolutions` defines the various resolutions to keep data
- `max-partitions` defines the number of partitions to use when
  creating consolidated tables
- `system-log-ttl` defines the TTL for system log tables. Set to 0 to disable.
  As these tables are partitioned by month, it's useless to use a too low value.
  The default value is 30 days. This requires a restart of ClickHouse.
- `networks` maps subnets to attributes. Attributes are `name`,
  `role`, `site`, `region`, and `tenant`. They are exposed as
  `SrcNetName`, `DstNetName`, `SrcNetRole`, `DstNetRole`, etc.
- `network-sources` fetch a remote source mapping subnets to
  attributes. This is similar to `networks` but the definition is
  fetched through HTTP. It accepts a map from source names to sources.
  Each source accepts the following attributes:
  - `url` is the URL to fetch
  - `method` is the method to use (`GET` or `POST`)
  - `headers` is a map from header names to values to add to the request
  - `proxy` says if we should use a proxy (defined through environment variables like `http_proxy`)
  - `timeout` defines the timeout for fetching and parsing
  - `interval` is the interval at which the source should be refreshed
  - `transform` is a [jq](https://stedolan.github.io/jq/manual/)
    expression to transform the received JSON into a set of network
    attributes represented as objects. Each object must have a
    `prefix` attribute and, optionally, `name`, `role`, `site`,
    `region`, and `tenant`. See the example provided in the shipped
    `akvorado.yaml` configuration file.
- `asns` maps AS number to names (overriding the builtin ones)
- `orchestrator-url` defines the URL of the orchestrator to be used
  by ClickHouse (autodetection when not specified)

The `resolutions` setting contains a list of resolutions. Each
resolution has two keys: `interval` and `ttl`. The first one is the
consolidation interval. The second is how long to keep the data in the
database. If `ttl` is 0, then the data is kept forever. If `interval`
is 0, it applies to the raw data (the one in the `flows` table). For
each resolution, a materialized view `flows_XXXX` is created with the
specified interval. It should be noted that consolidated tables do not
contain information about source/destination IP addresses and ports.
That's why you may want to keep the interval-0 table data a bit
longer. *Akvorado* will still use the consolidated tables if the query
do not require the raw table, for performance reason.

Here is the default configuration:

```yaml
resolutions:
  - interval: 0
    ttl: 360h  # 15 days
  - interval: 1m
    ttl: 168h  # 1 week
  - interval: 5m
    ttl: 2160h # 3 months
  - interval: 1h
    ttl: 8760h # 1 year
```

## Console service

The main components of the console service are `http`, `console`,
`authentication` and `database`. `http` accepts the [same configuration](#http)
as for the inlet service.

The console itself accepts the following keys:

 - `default-visualize-options` to define default options for the
   "visualize" tab and the second one defines the widgets to display
   on the home page (among `src-as`, `dst-as`, `src-country`,
   `dst-country`, `exporter`, `protocol`, `etype`, `src-port`, and
   `dst-port`)
 - `homepage-top-widgets` to define the widgets to display on the home page
 - `dimensions-limit` to set the upper limit of the number of returned dimensions
 - `cache-ttl` sets the time costly requests are kept in cache

Here is an example:

```yaml
console:
  homepage-top-widgets: [src-as, src-country, etype]
  default-visualize-options:
    start: 1 day ago
    end: now
    filter: InIfBoundary = external
    dimensions:
      - ExporterName
```

### Authentication

The console does not store user identities and is unable to
authenticate them. It expects an authenticating proxy will add some
headers to the API endpoints:

- `Remote-User` is the user login,
- `Remote-Name` is the user display name,
- `Remote-Email` is the user email address,
- `X-Logout-URL` is a link to the logout link.

Only the first header is mandatory. The name of the headers can be
changed by providing a different mapping under the `headers` key. It
is also possible to modify the default user (when no header is
present) by tweaking the `default-user` key:

```yaml
auth:
  headers:
    login: Remote-User
    name: Remote-Name
    email: Remote-Email
    logout-url: X-Logout-URL
  default-user:
    login: default
    name: Default User
```

To prevent access when not authenticated, the `login` field for the
`default-user` key should be empty.

There are several systems providing user management with all the bells
and whistles, including OAuth2 support, multi-factor authentication
and API tokens. Here is a short selection of solutions able to act as
an authenticating reverse-proxy for Akvorado:

- [Authelia](https://www.authelia.com/)
- [Authentik](https://goauthentik.io/)
- [Gluu](https://gluu.org/)
- [Keycloak](https://www.keycloak.org/)
- [Ory](https://www.ory.sh/), notably Kratos, Hydra and Oathkeeper
- [Casdoor](https://casdoor.org/)
- [Zitadel](https://zitadel.com/) combined with [OAuth2 Proxy](https://zitadel.com/docs/examples/identity-proxy/oauth2-proxy)

There also exist simpler solutions only providing authentication:

- [OAuth2 Proxy](https://oauth2-proxy.github.io/oauth2-proxy/), associated with [Dex](https://dexidp.io/)
- [Ory](https://www.ory.sh), notably Hydra and Oathkeeper

### Database

The console stores some data, like per-user filters, into a relational
database. When the database is not configured, data is only stored in
memory and will be lost on restart. Currently, the only accepted
driver is SQLite.

```yaml
database:
  driver: sqlite
  dsn: /var/lib/akvorado/console.sqlite
```

The database configuration also accepts a `saved-filters` key to
populate the database with the provided filters. Each filter should
have a `description` and a `content`:

```yaml
database:
  saved-filters:
    - description: From Netflix
      content: InIfBoundary = external AND SrcAS = AS2906
```

## Demo exporter service

For testing purpose, it is possible to generate flows using the demo
exporter service. It features a NetFlow generator, a simple SNMP
agent and a BMP exporter.

```yaml
snmp:
  name: exporter1.example.com
  interfaces:
    10: "Transit: Telia"
    11: "IX: AMSIX"
    20: "core"
    21: "core"
  listen: :161
bmp:
  target: 127.0.0.1:10179
  routes:
    - prefixes: 192.0.2.0/24,2a01:db8:cafe:1::/64
      aspath: 64501
      communities: 65401:10,65401:12
flows:
  samplingrate: 50000
  target: 127.0.0.1:2055
  flows:
    - per-second: 0.2
      in-if-index: 10
      out-if-index: 20
      peak-hour: 16h
      multiplier: 3
      src-port: 0
      dst-port: 80
      protocol: tcp
      size: 1300
      dst-net: 192.0.2.0/24
      dst-as: 64501
      src-net: 198.38.120.0/23
      src-as: 2906
```

In the `snmp` section, all fields are mandatory. The `interfaces`
section maps interface indexes to their descriptions. In the `bmp`
session, for each set of prefixes, the `aspath` is mandatory, but the
`communities` are optional. In the `flows` section, all fields are
mandatory. Have a look at the provided `akvorado.yaml` configuration
file for a more complete example. As generating many flows is quite
verbose, it may be useful to rely on [YAML anchors][] to avoid
repeating a lot of stuff.

[YAML anchors]: https://www.linode.com/docs/guides/yaml-anchors-aliases-overrides-extensions/
[clickhouse documentation]: https://clickhouse.com/docs/en/engines/table-engines/integrations/kafka/#table_engine-kafka-creating-a-table
