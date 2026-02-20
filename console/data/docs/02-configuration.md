# Configuration

The orchestrator service is configured through YAML files (provided in the
`config/` directory) and includes the configuration of the other services.

> [!TIP]

> Other services query the orchestrator through HTTP on startup to get their
> configuration. By default, the orchestrator restarts automatically if it
> detects a configuration change, but this may fail if there is a configuration
> error. Look at the logs of the orchestrator service or restart it if you think
> a configuration change is not applied.

You can get the default configuration with `docker compose run --rm --no-deps
akvorado-orchestrator orchestrator --dump --check /dev/null`. Note that
some sections are generated from the configuration of other sections. It is
better to not use the generated configuration as a base for your configuration.
Write durations as strings, like `10h20m` or `5s`. Valid time units are `ms`,
`s`, `m`, and `h`.

You can also override configuration settings with environment variables. Remove
any `-` from key names and use `_` for nesting. Then, add the prefix
`AKVORADO_CFG_ORCHESTRATOR_`. Let's consider this configuration file:

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

The orchestrator service has its own configuration and the configuration for the
other services. The configuration for each service is under a key with the same
name as the service (`inlet`, `outlet`, and `console`). For each service, you
can provide a list of configurations. A service can request a specific
configuration by adding an index to the configuration URL. If the index does not
match a configuration, the first configuration is used.

Each service has several functional components. Each component has a section in
the configuration file with the same name.

## Inlet service

Configure this service under the `inlet` key. The inlet service receives
NetFlow/IPFIX/sFlow packets and sends them to Kafka. Its main components are
`flow` and `kafka`.

### Flow

The `flow` component handles incoming flows. Use the `inputs` key to define a
list of inputs for incoming flows. The flows are put into protobuf messages and
sent to Kafka without being parsed.

Each input has a `type` and a `decoder`. For `decoder`, `netflow` and `sflow`
are supported. For `type`, `udp` and `file` are supported.

For all available inputs, the following options are available:

- `use-src-addr-for-exporter-addr` to be set to true if the source IP of the
  received flow packet should be used as the exporter address.
- `timestamp-source` to choose the source of the timestamp for each flow: `udp`
  to use the receive time of the UDP packet (the default), `netflow-packet` to
  extract the timestamp from the NetFlow/IPFIX header, `netflow-first-switched`
  to use the “first switched” field from NetFlow/IPFIX.
- `decapsulation-protocol` to look inside a tunneling protocol. The supported
  protocols are `none` (the default), `ipip` (both IPv4 and IPv6), `gre`
  (version 0), `vxlan` (UDP port 4789), and `srv6` (DT4, DT6, DT46, DX4, DX6 are
  supported, not DX2, nor DT2). This requires the presence of a sampled packet
  for sFlow or the use of [IPFIX
  315](https://datatracker.ietf.org/doc/html/rfc7133). If there is a protocol
  mismatch, the packet will be dropped.
- `rate-limit` to set the maximum number of flows per second per exporter. When
  the rate is exceeded, excess flows are dropped before being written to
  ClickHouse. The sampling rate of the remaining flows is adjusted to compensate
  for the dropped flows. Flows are still sent through Kafka. Set to `0` to
  disable (the default).

For the UDP input, you can use the following keys:

- `listen`: set the listening endpoint.
- `workers`: set the number of workers to listen to the socket.
- `receive-buffer`: set the size of the kernel's incoming buffer for each listening socket.

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
```

Use the `file` input for testing only. It has a `paths` key to define the files
to read. These files are continuously added to the processing pipeline. For
example:

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
```

Without configuration, *Akvorado* listens for incoming NetFlow/IPFIX and sFlow
flows on a random port. Check the logs to see which port is used.

### Kafka

The inlet service sends received flows to a Kafka topic using the [protocol
buffers format][].

[protocol buffers format]: https://developers.google.com/protocol-buffers

The following keys are accepted:

- `topic`, `brokers`, and `tls` are described in the configuration for the
  [orchestrator service](#kafka-2). Their values are copied from the
  orchestrator configuration, unless you set `brokers` explicitly.
- `compression-codec` defines the compression codec for messages: `none`,
  `gzip`, `snappy`, `lz4` (default), or `zstd`.
- `queue-size` defines the maximum number of messages to buffer for Kafka.
- `load-balance` defines the load balancing algorithm for flows accross Kafka
  partitions. The default value is `random`: each flow is assigned a random
  partition, ensuring an even distribution. The other possible value is
  `by-exporter`: all flows from a given exporter is assigned to a single
  partition. This setting can be important if you have several outlets and IPFIX
  or NetFlow: each outlet needs to receive the templates before decoding flows
  and this is less likely when using `random`.

A version number is automatically added to the topic name. This is to prevent
problems if the protobuf schema changes in a way that is not
backward-compatible.

## Outlet service

Configure this service under the `outlet` key. The outlet service takes flows
from Kafka, parses them, adds metadata and routing information, and sends them
to ClickHouse. Its main components are `kafka`, `metadata`, `routing`, and `core`.

### Kafka

The outlet's Kafka component takes flows from the Kafka topic. The following
keys are accepted:

- `topic`, `brokers`, and `tls` are described in the configuration for the
  [orchestrator service](#kafka-2). Their values are copied from the
  orchestrator configuration, unless you set `brokers` explicitly.
- `consumer-group` defines the consumer group ID for Kafka consumption.
- `fetch-min-bytes` defines the minimum number of bytes to fetch from Kafka.
- `fetch-max-wait-time` defines the maximum time to wait for the minimum
  number of bytes to become available.
- `min-workers` defines the minimum number of Kafka workers to use.
- `max-workers` defines the maximum number of Kafka workers to use (it should
  not be more than the number of partitions for the topic, as defined in
  `kafka`→`num-partitions`)
- `worker-increase-rate-limit` defines the duration before increasing the
  number of workers.
- `worker-decrease-rate-limit` defines the duration before decreasing the
  number of workers.

The number of running workers depends on the load of the ClickHouse
component. The number of workers is adjusted to stay below
`maximum-batch-size`. Do not set `max-workers` too high, as it can
increase the load on ClickHouse. The default value of 8 is usually fine.

### Routing

The routing component can get the source and destination AS numbers, AS paths,
and communities. Not all exporters provide this information. Currently, the
default provider is BMP. *Akvorado* tries to select the best route using the
next hop from the flow. If it is not found, it will use any other next hop.

The component has a `provider` key that defines the provider
configuration. Inside the provider configuration, the `type` key defines the
provider type. `bmp` and `bioris` are currently supported. The remaining
keys are specific to the provider.

#### BMP provider

For the BMP provider, the following keys are accepted:

- `listen` specifies the IP address and port to listen for incoming connections
  (default port is 10179).
- `rds` is a list of route distinguishers to accept. Use 0 to accept routes
  without a route distinguisher.
- `collect-asns` defines if origin AS numbers should be collected.
- `collect-aspaths` defines if AS paths should be collected.
- `collect-communities` defines if communities should be collected. It supports
  regular and large communities, but not extended communities.
- `keep` defines how long to keep routes from a terminated BMP
  connection.
- `receive-buffer` is the size of the kernel receive buffer in bytes for each
  established BMP connection.

If you do not need AS paths and communities, you can disable them to save memory
and disk space in ClickHouse.

*Akvorado* supports receiving AdjRIB-in, with or without
filtering. It can also work with a LocRIB.

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

> [!NOTE]
> With many routes, BMP can have performance issues when a peer disconnects.
> If you do not need full accuracy, limit the number of BMP peers and
> export the LocRIB. These issues will be fixed in a future release.

#### BioRIS provider

As an alternative to the internal BMP, you can connect to an existing [bio-rd
RIS](https://github.com/bio-routing/bio-rd/tree/master/cmd/ris) instance. It
accepts the following keys:

- `ris-instances` is a list of instances.
- `timeout` defines how long to wait for an answer from a RIS instance.
- `refresh` defines how long to wait between refreshing the list of routers.

Each instance accepts the following keys:

- `grpc-addr` is the address and port of a RIS instance.
- `grpc-secure` tells if a connection should be set using TLS.
- `vrf` (as a string) or `vrf-id` (as an ID) defines which VRF to look up.

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

BioRIS queries the RIB of the router that sent the flow. If this router's
RIB is not available in any of the known RIS instances, another router is used
as a fallback. After the router ID is determined, BioRIS queries one of the
RIS instances that has the RIB.

BioRIS can set the prefix, AS, AS Path, and communities for the flow.

### Metadata

Flows only include interface indexes. To associate them with an interface name
and description, metadata is retrieved from the exporting routers. A cache is
used. Several providers are available to poll metadata. The
following keys are accepted:

- `cache-duration` defines how long to keep data in the cache.
- `cache-refresh` defines how long to wait before updating an entry
  by polling it.
- `cache-check-interval` defines how often to check if cached data is
  about to expire or needs an update.
- `cache-persist-file` defines where to store cached data on shutdown and
  read it back on startup.
- `query-timeout` defines how long to wait for a provider to answer a query.
- `initial-delay` defines how long to wait after starting before applying the
  standard query timeout.
- `providers` defines the provider configurations.

Because flows missing any interface information are discarded, persisting the cache
is useful to quickly handle incoming flows.

The `providers` key contains the provider configurations. For each, the
provider type is defined by the `type` key. When using several providers, they
are queried in order and the process stops on the first one that accepts the query.
Currently, only the `static` provider can skip a query. Therefore, you
should put it first.

#### SNMP provider

The `snmp` provider accepts these configuration keys:

- `credentials` is a map from exporter subnets to credentials. Use `::/0` to set
  the default value. For SNMPv2, it accepts the `communities` key. It is a single
  community or a list of communities. In the latter case, each community
  is tried in order for all requests. For SNMPv3, it accepts the following keys:
  `user-name`, `authentication-protocol` (`none`, `MD5`, `SHA`, `SHA224`,
  `SHA256`, `SHA384`, and `SHA512` are accepted), `authentication-passphrase`
  (if the previous value was set), `privacy-protocol` (`none`, `DES`, `AES`,
  `AES192`, `AES256`, `AES192-C`, and `AES256-C` are accepted, the latters being
  Cisco variants), `privacy-passphrase` (if the previous value was set), and
  `context-name`. `AES` means AES with a 128-bit key and `SHA` is SHA1.
- `ports` is a map from exporter subnets to the SNMP port to use for polling
  exporters in the provided subnet.
- `agents` is a map from exporter IPs to agent IPs. When there is no match, the
  exporter IP is used. Other options still use the exporter IP as a key,
  not the agent IP.
- `poller-retries` is the number of retries for unsuccessful SNMP requests.
- `poller-timeout` defines how long the poller should wait for an answer.

*Akvorado* uses SNMPv2 if `communities` is present and SNMPv3 if `user-name` is
present. You need one of them.

For example, with SNMPv2, you can try both `private` and `@private` SNMPv2
communities:

```yaml
metadata:
  workers: 10
  providers:
    - type: snmp
      credentials:
        ::/0:
          communities:
            - private
            - "@private"
```

And with SNMPv3:

```yaml
metadata:
  workers: 10
  providers:
    - type: snmp
      credentials:
        ::/0:
          user-name: monitoring
          authentication-protocol: SHA
          authentication-passphrase: "d$rkSec"
          privacy-protocol: AES192
          privacy-passphrase: "Cl0se"
```

#### gNMI provider

The `gnmi` provider polls an exporter using gNMI. It accepts these keys:

- `targets` is a map from exporter subnets to target IPs. When there is no match,
  the exporter IP is used. Other options still use the exporter IP as a
  key, not the target IP.
- `ports` is a map from exporter subnets to the gNMI port to use for polling
  exporters in the provided subnet.
- `set-target` is a map from exporter subnets to a boolean that specifies if the target
  name should be set in the gNMI path prefix. In this case, it is set to the
  exporter IP address. This is useful if the selected target is a gNMI gateway.
- `authentication-parameters` is a map from exporter subnets to authentication
  parameters for gNMI targets. Authentication parameters accept these keys:
  `username`, `password`, and `tls` (which takes the same keys as for
  [Kafka](#kafka-2)).
- `models` is the list of models to use to get information from a target. Each
  model is tried, and if a target supports all the paths, it is selected. The
  models are tried in the order they are declared. If you want to keep the
  built-in models, use the special string `defaults`.
- `timeout` defines how long to wait for an answer from a target.
- `minimal-refresh-interval` is the minimum time a collector will wait before
  polling a target again.

For example:

```yaml
metadata:
 providers:
  type: gnmi
  authentication-parameters:
   ::/0:
    username: admin
    password: NokiaSrl1!
    skip-verify: true
```

Unlike SNMP, a single metadata worker is sufficient for gNMI.

The gNMI provider uses "subscribe once" to poll for information from the
target. This should be compatible with most targets.

A model accepts these keys:

- `name` for the model name (e.g., `Nokia SR Linux`).
- `system-name-paths` is a list of paths to get the system name (e.g.,
  `/system/name/host-name`).
- `if-index-paths` is a list of paths to get interface indexes.
- `if-name-keys` is a list of keys where you can find the name of an interface in
  the paths returned for interface indexes (e.g., `name` or `port-id`).
- `if-name-paths` is a list of paths to get interface names. These paths take
  precedence over the previous key if found.
- `if-description-paths` is a list of paths to get interface descriptions.
- `if-speed-paths` is a list of paths to get interface speeds. For
  this key, a path is defined by two keys: `path` for the gNMI path and `unit`
  for the unit on how to interpret the value. A unit can be `bps` (bits per
  second), `mbps` (megabits per second), `ethernet` (OpenConfig `ETHERNET_SPEED`
  like `SPEED_100GB`), or `human` (human-readable format like `10G` or `100M`).

The currently supported models are:
- Nokia SR OS
- Nokia SR Linux
- OpenConfig
- IETF

#### Static provider

The `static` provider accepts an `exporters` key that maps exporter subnets to
an exporter configuration. An exporter configuration is a map:

- `name` is the name of the exporter.
- `default` is the default interface when no match is found.
- `ifindexes` is a map from interface indexes to an interface.
- `skip-missing-interfaces` defines whether the exporter should process only
  the interfaces defined in the configuration and leave the rest to the next
  provider. This conflicts with the `default` setting.

An interface has a `name`, a `description`, and a `speed`.

For example, to add an exception for `2001:db8:1::1` and then use SNMP for
other exporters:

```yaml
metadata:
  providers:
    - type: static
      exporters:
        2001:db8:1::1:
          name: exporter1
          skip-missing-interfaces: true
          ifindexes:
            10:
              name: Gi0/0/10
              description: PNI Netflix
              speed: 1000
            11:
              name: Gi0/0/15
              description: PNI Google
              speed: 1000
    - type: snmp
      communities:
        ::/0: private
```

The `static` provider also accepts an `exporter-sources` key, which fetches a
remote source that maps subnets to attributes. This is similar to `exporters`,
but the definition is fetched through HTTP. It accepts a map from source names to
sources. Each source accepts these attributes:

- `url` is the URL to fetch.
- `tls` defines the TLS configuration to connect to the source (it uses the same
  configuration as for [Kafka](#kafka-2), be sure to set `enable` to `true`)
- `method` is the method to use (`GET` or `POST`).
- `headers` is a map of header names to values to add to the request.
- `proxy` defines if a proxy should be used (defined with environment variables
  like `http_proxy`).
- `timeout` defines the timeout for fetching and parsing.
- `interval` is the interval at which the source should be refreshed.
- `transform` is a [jq](https://stedolan.github.io/jq/manual/) expression that
  transforms the received JSON into a set of attributes represented as objects.
  Each object should have these keys: `exporter-subnet`, `default` (with the
  same structure as a static configuration), and `interfaces`. The latter is a
  list of interfaces, where each interface has an `ifindex`, a `name`, a
  `description`, and a `speed`.

For example:

```yaml
metadata:
  providers:
    type: static
    exporter-sources:
      gostatic:
        url: http://gostatic:8043/my-exporters.json
        interval: 10m
        transform: .exporters[]
```

### Core

The core component processes flows from Kafka, queries the `metadata` component to
enrich the flows with additional information, and classifies
exporters and interfaces into groups with a set of classification
rules. It also enforces per-exporter rate limiting as configured in the inlet.

The following configuration keys are accepted:

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
- `asn-providers` defines the source list for AS numbers. The available sources
  are `flow`, `flow-except-private` (use information from flow except if the ASN
  is private), `flow-except-default-route` (use information from flow except if
  the NetMask is the default route), `routing`, `routing-except-private`, and
  `geo-ip`. The default value is `flow`, `routing`, `geo-ip`. `geo-ip` should
  only be used at the end as there is no fallback possible.
- `net-providers` defines the sources for prefix lengths and nexthop. `flow` uses the value
  provided by the flow message (if any), while `routing` looks it up using the BMP
  component. If multiple sources are provided, the value of the first source
  providing a non-default route is taken. The default value is `flow` and `routing`.

#### Classification

Classifier rules are written in a language called [Expr][].

Interface classifiers gets exporter and interface-related information as input.
If they can make a decision, they should invoke one of the `Classify()`
functions with the target element as an argument. Once classification is done
for an element, it cannot be changed by a subsequent rule. All strings are
normalized (lower case, special chars removed).

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
normalized (lower case, special chars removed).

Each `Classify()` function, with the exception of `ClassifyExternal()`
and `ClassifyInternal()` have a variant ending with `Regex` which
takes a string and a regex before the original string and do a regex
match. The original string is expanded using the matching parts of the
regex. The syntax is the one [from Go][]. If you want to use Perl
character classes, such as `\d` or `\w`, you need to escape the
backslash character: `\\d` and `\\w`. To test your regex, you can use
a site like [regular expressions 101][]. Be sure to use the "Golang"
flavor.

[regular expressions 101]: https://regex101.com/

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

The first rule says “extract the connectivity (transit, pni, ppni or ix) from
the interface description, and if successful, use the second part of the
description as the provider, and if successful, considers the interface as an
external one”. The second rule says “if an interface was not classified as
external or internal, consider it as an internal one.”

When the description is `Transit: Cogent 1-3834938493`, the first rule will put
`transit` into the connectivity field (check with
[regex101.com](https://regex101.com/r/FPITQE/1)), then `cogent` in the provider
field (check with [regex101.com](https://regex101.com/r/jnzhSv/1)), and classify
the interface as external. If the interface is the input one, you'll get
`InIfConnectivity` set to `transit`, `InIfProvider` set to `cogent`, and
`InIfBoundary` set to `external`.

Exporter classifiers gets the classifier IP address and its hostname. Like the
interface classifiers, they should invoke one of the `Classify()` functions to
make a decision:

- `Exporter.IP` for the exporter IP address
- `Exporter.Name` for the exporter name
- `ClassifyGroup()` to classify the exporter to a group
- `ClassifyRole()` to classify the exporter for a role (`edge`, `core`)
- `ClassifySite()` to classify the exporter to a site (`paris`, `berlin`, `newyork`)
- `ClassifyRegion()` to classify the exporter to a region (`france`, `italy`, `caraibes`)
- `ClassifyTenant()` to classify the exporter to a tenant (`team-a`, `team-b`)
- `Reject()` to reject the flow
- `Format()` to format a string: `Format("name: %s", Exporter.Name)`

Here is an example, assuming routers are named `th2-ncs55a1-1.example.fr` or
`milan-ncs5k8-2.example.it`:

```yaml
exporter-classifiers:
  - ClassifySiteRegex(Exporter.Name, "^([^-]+)-", "$1")
  - Exporter.Name endsWith ".it" && ClassifyRegion("italy")
  - Exporter.Name matches "^(washington|newyork).*" && ClassifyRegion("usa")
  - Exporter.Name endsWith ".fr" && ClassifyRegion("france")
```

You can check the result of the classification with the following command:

```console
$ curl -s http://127.0.0.1:8080/api/v0/console/widget/flow-last | jq .
{
  "Bytes": 1500,
  "Dst1stAS": 64501,
[...]
  "ExporterName": "dc3-edge1.example.com",
  "ExporterRegion": "europe",
  "ExporterRole": "edge",
  "ExporterSite": "dc3",
  "ExporterTenant": "acme",
[...]
  "InIfBoundary": "external",
  "InIfConnectivity": "transit",
  "InIfDescription": "Transit: Tata",
  "InIfName": "Gi0/0/0/10",
  "InIfProvider": "tata",
[...]
```

[expr]: https://expr-lang.org/docs/language-definition
[from Go]: https://github.com/google/re2/wiki/Syntax

### ClickHouse

The ClickHouse component pushes data to ClickHouse. There are three settings that
are configurable:

- `maximum-batch-size` defines how many flows to send to ClickHouse in a single batch at most
- `minimum-wait-time` defines how long to wait before sending an incomplete batch
- `grace-period` defines how long to wait when flushing data to ClickHouse on shutdown

These numbers are per-worker (as defined in the Kafka component). A worker will
send a batch of size at most `maximum-batch-size` at least every
`maximum-wait-time`. ClickHouse is more efficient when the batch size is large.
The default value is 100 000 and allows ClickHouse to handle incoming flows
efficiently.

### Flow

The flow component decodes flows received from Kafka. There is only one setting:

- `state-persist-file` defines the location of the file to save the state of the
  flow decoders and read it back on startup. It is used to store IPFIX/NetFlow
  templates and options.

## Orchestrator service

The three main components of the orchestrator service are `schema`,
`clickhouse`, and `kafka`. The `automatic-restart` directive tells the
orchestrator to watch for configuration changes and restart if there are any. It
is enable by default.

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
version -d`. Disabling a column won't delete existing data.

It is also possible to make some columns available on the main table only
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

#### Custom dictionaries

You can add custom dimensions to be looked up via a dictionary. This is useful
to enrich your flow with additional information not possible to get in the
classifier. This works by providing the database with a CSV file containing the
values.

```yaml
schema:
  custom-dictionaries:
    ips:
      layout: complex_key_hashed
      keys:
        - name: addr
          type: String
      attributes:
        - name: role
          type: String
          default: DefaultRole
          label: IPRole
      source: /etc/akvorado/ips_annotation.csv
      dimensions:
        - SrcAddr
        - DstAddr
```

This example expects a CSV file named `ips_annotation.csv` (when using Docker,
put it in the `config/` directory) with the following format:

```csv
addr,role
2001:db8::1,ExampleRole
```

> [!NOTE]
> For IPv4 addresses, you need to use `::ffff:a.b.c.d`. Internally, Akvorado
> uses only IPv6 addresses.

If `SrcAddr` has the value `2001:db8::1` (matches the key), the dimension
`SrcAddrIPRole` will be set to `ExampleRole`. Independently, if `DstAddr` has
the value `2001:db8::1`, the dimension `DstAddrIPRole` will be set to
`ExampleRole`.

All other IPs will get "DefaultRole" in their `SrcAddrIPRole`/`DstAddrIPRole`
dimension.

The `label` and `default` keys are optional.

It is possible to add the same dictionary to multiple dimensions, usually for
the "Input" and "Output"-direction.

By default, the value of the key tries to match a dimension. For multiple keys,
it is necessary to explicitly specify the dimension name to match by either
specifing `match-dimension` or `match-dimension-suffix`:

```yaml
schema:
  custom-dictionaries:
    interfaces:
      layout: complex_key_hashed
      dimensions:
        - OutIf
        - InIf
      keys:
        - name: agent
          type: String
          # CSV column “agent” matches the ExporterAddress dimension
          match-dimension: ExporterAddress
        - name: interface
          type: String
          # CSV column “interface” matches matches either OUtIfName or InIfName
          match-dimension-suffix: Name
      attributes:
        - name: information # OutIfInformation/InIfInformation
          type: String
          # No default. If no match of both agent and interface, the dimension is empty
      source: /etc/akvorado/interfaces.csv
```

### Kafka

The Kafka component creates or updates the Kafka topic to receive
flows. It accepts the following keys:

- `brokers` specifies the list of brokers to use to bootstrap the
  connection to the Kafka cluster
- `tls` defines the TLS configuration to connect to the cluster
- `sasl` defines the SASL configuration to connect to the cluster
- `topic` defines the base topic name
- `manage-topic` controls whether the orchestrator should create or update the
  Kafka topic. Can be set to `false` when Kafka is managed externally.
- `topic-configuration` describes how the topic should be configured

The following keys are accepted for the TLS configuration:

- `enable` should be set to `true` to enable TLS.
- `skip-verify` can be set to `true` to skip checking server certificate (not recommended).
- `ca-file` gives the location of the file containing the CA certificate in PEM
  format to check the server certificate. If not provided, the system
  certificates are used instead.
- `cert-file` and `key-file` defines the location of the client certificate pair
  in PEM format to authenticate to the broker. If the first one is empty, no
  client certificate is used. If the second one is empty, the key is expected to
  be in the certificate file.

The following keys are accepted for SASL configuration:

- `username` and `password` enables SASL authentication with the
  provided user and password.
- `algorithm` tells which SASL mechanism to use for authentication. This
  can be `none`, `plain`, `scram-sha256`, `scram-sha512`, or `oauth`. This should not be
  set to none when SASL is used.
- `oauth-token-url` defines the URL to query to get a valid OAuth token (in this
  case, `username` and `password` are used as client credentials).
- `oauth-scopes` defines the list of scopes to request for the OAuth token.

The following keys are accepted for the topic configuration:

- `num-partitions` for the number of partitions
- `replication-factor` for the replication factor
- `config-entries` is a mapping from configuration names to their values
- `config-entries-strict-sync` for the configuration in-sync policy

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
    config-entries-strict-sync: true
```

Another useful setting is `retention.bytes` to limit the size of a
partition in bytes too (divide it by the number of partitions to have
a limit for the topic).

Currently, the orchestrator service won't update the replication
factor. 
By default, the configuration entries are kept in sync with the content of
the configuration file, except if you disable the `config-entries-strict-sync`,
the existing non-listed overrides won't be removed from topic configuration entries.

### ClickHouse database

The ClickHouse database component contains the settings to connect to the
ClickHouse database. The following keys should be provided inside
`clickhousedb`:

- `servers` defines the list of ClickHouse servers to connect to
- `username` is the username to use for authentication
- `password` is the password to use for authentication
- `database` defines the database to use to create tables
- `cluster` defines the cluster for replicated and distributed tables, see the next section for more information
- `tls` defines the TLS configuration to connect to the database (it uses the same configuration as for [Kafka](#kafka-2))

### ClickHouse

The ClickHouse component exposes some useful HTTP endpoints to
configure a ClickHouse database. It also provisions and keep
up-to-date a ClickHouse database. The following keys can be
provided inside `clickhouse`:

- `resolutions` defines the various resolutions to keep data
- `max-partitions` defines the number of partitions to use when
  creating consolidated tables
- `networks` maps subnets to attributes. Attributes are `name`, `role`, `site`,
  `region`, and `tenant`. They are exposed as `SrcNetName`, `DstNetName`,
  `SrcNetRole`, `DstNetRole`, etc. It is also possible to override GeoIP
  attributes `city`, `state`, `country`, and `ASN`.
- `network-sources` fetch a remote source mapping subnets to attributes. This is
  similar to `networks` but the definition is fetched through HTTP. It accepts a
  map from source names to sources. Each source accepts the following
  attributes:
  - `url` is the URL to fetch
  - `tls` defines the TLS configuration to connect to the source (it uses the
    same configuration as for [Kafka](#kafka-2), be sure to set `enable` to
    `true`)
  - `method` is the method to use (`GET` or `POST`)
  - `headers` is a map from header names to values to add to the request
  - `proxy` says if we should use a proxy (defined through environment variables like `http_proxy`)
  - `timeout` defines the timeout for fetching and parsing
  - `interval` is the interval at which the source should be refreshed
  - `transform` is a [jq](https://stedolan.github.io/jq/manual/) expression to
    transform the received JSON into a set of network attributes represented as
    objects. Each object must have a `prefix` attribute and, optionally, `name`,
    `role`, `site`, `region`, `tenant`, `city`, `state`, `country`, and `asn`.
    See the example provided in the shipped `akvorado.yaml` configuration file.
- `asns` maps AS number to names (overriding the builtin ones)
- `orchestrator-url` defines the URL of the orchestrator to be used
  by ClickHouse (autodetection when not specified)
- `orchestrator-basic-auth` enables basic authentication to access the
  orchestrator URL. It takes two attributes: `username` and `password`.
- `skip-migrations` controls whether to skip ClickHouse schema management. Can
  be set to `true` when the schema is managed externally or by another
  orchestrator. The outlet requires the schema to match the expected structure:
  schema mismatches may cause write errors.

The `resolutions` setting contains a list of resolutions. Each
resolution has two keys: `interval` and `ttl`. The first one is the
consolidation interval. The second is how long to keep the data in the
database. If `ttl` is 0, then the data is kept forever. If `interval`
is 0, it applies to the raw data (the one in the `flows` table). For
each resolution, a materialized view `flows_DDDD` is created with the
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

If you want to tweak the values, start from the default configuration. Most of
the disk space is taken by the main table (`interval: 0`) and you can reduce its
TTL if it's too big for your usage. Check the [operational
documentation](04-operations.md#space-usage) for information on how to check
disk usage. If you remove an existing interval, it is not removed from the
ClickHouse database and will continue to be populated.

It is mandatory to specify a configuration for `interval: 0`.

When specifying a cluster name with `cluster`, the orchestrator will manage a
set of replicated and distributed tables. No migration is done between the
cluster and the non-cluster modes, therefore, you shouldn't change this setting
without also changing the database. If you already have an existing setup, this
means you need to start from scratch and copy data. There is currently no
instruction for that, but it's mostly a matter of copying `flows` table to
`flows_local`, and `flows_DDDD` (where `DDDD` is an interval) tables to
`flows_DDDD_local`.

### GeoIP

The `geoip` directive allows one to configure two databases using the [MaxMind
DB file format][], one for AS numbers, one for countries/cities. It accepts the
following keys:

- `asn-database` tells the paths to the ASN database
- `geo-database` tells the paths to the geo database (country or city)
- `optional` makes the presence of the databases optional on start
  (when not present on start, the component is just disabled)

[MaxMind DB file format]: https://maxmind.github.io/MaxMind-DB/

If the files are updated while *Akvorado* is running, they are automatically
refreshed. For a given database, the latest paths override the earlier ones.

## Console service

The main components of the console service are `console`, `authentication` and
`database`.

The console itself accepts the following keys:

 - `default-visualize-options` to define default options for the "visualize"
   tab. It takes the following keys: `graph-type` (one of `stacked`,
   `stacked100`, `lines`, `grid`, or `sankey`), `start`, `end`, `filter`,
   `dimensions` (a list), `limit`, `limitType`, `bidirectional` (a bool), `previous-period`
   (a bool)
 - `homepage-top-widgets` to define the widgets to display on the home page
   (among `src-as`, `dst-as`, `src-country`, `dst-country`, `exporter`,
   `protocol`, `etype`, `src-port`, and `dst-port`)
 - `dimensions-limit` to set the upper limit of the number of returned dimensions
 - `cache-ttl` sets the time costly requests are kept in cache
 - `homepage-graph-filter` sets the filter for the graph on the homepage
    (default: `InIfBoundary = 'external'`). This is a SQL expression, passed
    into the clickhouse query directly. It can also be empty, in which case the
    sum of all flows captured will be displayed.
 - `homepage-graph-timerange` sets the time range to use for the graph on the
   homepage. It defaults to 24 hours.

It also takes a `clickhouse` key, accepting the [same
configuration](#clickhouse-database) as the orchestrator service. These keys are
copied from the orchestrator, unless `servers` is set explicitely.

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
- `X-Logout-URL` is a link to the logout link,
- `X-Avatar-URL` is a link to the avatar image.

Only the first header is mandatory. The name of the headers can be changed by
providing a different mapping under the `headers` key. It is also possible to
modify the default user (when no header is present) by tweaking the
`default-user` key. If logout URL or avatar URL is not provided in the headers,
it is possible to provide them as `logout-url` and `avatar-url`. In this case,
they can be templated with `.Login`, `.Name`, `.Email`, `.LogoutURL`, and
`.AvatarURL`.

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
  avatar-url: "https://avatars.githubusercontent.com/{{ .Login }}?s=80"
  logout-url: "{{ if .LogoutURL }}{{ .LogoutURL }}{{ else }}/logout{{ end }}"
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
- [Zitadel](https://zitadel.com/)

There also exist simpler solutions only providing authentication:

- [OAuth2 Proxy](https://oauth2-proxy.github.io/oauth2-proxy/), associated with [Dex](https://dexidp.io/)
- [Ory](https://www.ory.sh), notably Hydra and Oathkeeper

Traefik can also be configured to [forward authentication requests][] to another
service, include [OAuth2 Proxy][] or [Traefik Forward Auth][]. Some examples are
present in `docker/docker-compose-local.yml`.

[forward authentication requests]: https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/forwardauth/
[oauth2 proxy]:
https://oauth2-proxy.github.io/oauth2-proxy/configuration/integration#configuring-for-use-with-the-traefik-v2-forwardauth-middleware
[traefik forward auth]: https://github.com/ItalyPaleAle/traefik-forward-auth

### Database

The console stores some data, like per-user filters, into a relational database.
When the database is not configured, data is only stored in memory and will be
lost on restart. Supported drivers are `sqlite`, `mysql`, and `postgresql`.

```yaml
database:
  driver: sqlite
  dsn: /var/lib/akvorado/console.sqlite
```

The `dsn` field for `sqlite` should be the path to the database. For `mysql`,
the format is `user:pass@tcp(hostname:3306)/dbname?charset=utf8mb4`. Check the
[documentation of the SQL
driver](https://github.com/go-sql-driver/mysql#dsn-data-source-name) for more
details. For `postgresql`, the format is `host=hostname port=5432 user=user
password=pass dbname=dbname sslmode=disable`. Check the [documentation of
libpq](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING)
for more details.

> [!IMPORTANT]
> With the Docker Compose setup, SQLite is configured by default with DSN
> `/run/akvorado/console.sqlite` using environment variable. To override this,
> uncomment the appropriate configuration snippet in
> `docker/docker-compose-local.yml`.

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

## Common configuration settings

All services also embeds an HTTP and a reporting component.

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
useful.

### Reporting

Reporting encompasses logging and metrics. Currently, as *Akvorado* is expected
to be run inside Docker, logging is done on the standard output and is not
configurable. As for metrics, they are reported by the HTTP component on the
`/api/v0/XXX/metrics` endpoint (where `XXX` is the service name) and there is
nothing to configure either.
