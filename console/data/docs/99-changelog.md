# Changelog

For each version, changes are listed in order of importance. Minor
changes are not listed here. Each change is mapped to a category
identified with a specific icon:

- 💥: breaking change
- ✨: new feature
- 🗑: removed feature
- 🔒: security fix
- 🩹: bug fix
- 🌱: miscellaneous change

## 1.9.1 - 2023-10-06

- 🌱 *console*: add filtering support for custom columns
- 🌱 *inlet*: update [Expr](https://expr.medv.io/), the language behind the
  classifiers: support for variables
- 🌱 *inlet*: support for RFC 7133 for IPFIX
- 🌱 *orchestrator*: improve performance when looking up for `SrcNetPrefix` and
  `DstNetPrefix` when these columns are materialized

## 1.9.0 - 2023-08-26

- 💥 *cmd*: use `AKVORADO_CFG_` as a prefix for environment variables used to
  modify configuration (`AKVORADO_CFG_ORCHESTRATOR_HTTP_LISTEN` instead of
  `AKVORADO_ORCHESTRATOR_HTTP_LISTEN`)
- 💥 *inlet*: `inlet`→`metadata`→`provider(snmp)`→`ports` is now a map from
  exporter subnets to ports, instead of a map from agent subnets to ports. This
  is aligned with how `communities` and `security-parameters` options behave.
- ✨ *inlet*: support for [IPinfo](https://ipinfo.io/) geo IP database and use
  it by default
- ✨ *inlet*: metadata retrieval is now pluggable. In addition to SNMP, it is
  now possible to set exporter names, interface names and descriptions directly
  in the configuration file. See `inlet`→`metadata`.
- ✨ *inlet*: routing information is now pluggable. See `inlet`→`routing`.
- ✨ *inlet*: BioRIS provider to retrieve routing information
- ✨ *inlet*: allow extraction of prefix length from routing information. See
  `inlet`→`core`→`net-providers`.
- ✨ *inlet*: add the following collected data (disabled by default):
  - `IPTTL`
  - `IPTos`
  - `FragmentID` and `FragmentOffset`
  - `TCPFlags`
  - `ICMPv4Type`, `ICMPv4Code`, `ICMPv6Type`, `ICMPv6Code`, `ICMPv4`, and `ICMPv6`
  - `NextHop`
- ✨ *orchestrator*: add custom dictionaries for additional flow hydration. See
  `orchestrator`→`schema`→`custom-dictionaries`. Currently, filtering on the
  generated data is not available.
- 🩹 *inlet*: fix Netflow processing when template is received with data
- 🩹 *inlet*: use sampling rate in Netflow data packet if available
- 🩹 *console*: fix display when using “%” units and interface speed is 0
- 🩹 *orchestrator*: create flows table with
  `allow_suspicious_low_cardinality_types` to ensure we can use
  `LowCardinality(IPv6)`.
- 🌱 *inlet*: update [Expr](https://expr.medv.io/), the language behind the
  classifiers: new builtins are available
- 🌱 *build*: minimum supported Node version is now 16
- 🌱 *docker*: move Docker-related files to `docker/`
- 🌱 *docker*: update ClickHouse to 23.3 (not mandatory)
- 🌱 *docker*: update to Zookeeper 3.8 (not mandatory)
- 🌱 *docker*: update to Kafka 3.5 (not mandatory, but there is also a configuration change)
- 🌱 *docker*: add healthchecks for Redis and Zookeeper
- 🌱 *console*: emphasize trajectory on Sankey graphs

## 1.8.3 - 2023-04-28

- 🩹 *docker*: ensure Kafka is not using KRaft by default
- 🩹 *console*: fix `SrcVlan` and `DstVlan` as a dimension
- 🌱 *orchestrator*: add `method` and `headers` to specify HTTP method and
  additional headers to use when requesting a network source

## 1.8.2 - 2023-04-08

- ✨ *orchestrator*: add an option to materialize a column instead of using an alias
- 🩹 *inlet*: fix caching when setting interface name or description

## 1.8.1 - 2023-03-04

- 🩹 *console*: fix subnet aggregation when IPv4 or IPv6 is set to its default value
- 🩹 *console*: fix `SrcNetPrefix`, `DstNetPrefix`, `PacketSize`, and `PacketSizeBucket` dimensions

## 1.8.0 - 2023-02-25

- 💥 *docker-compose*: the configuration files are now shipped in a `config/`
  directory: you need to move your `akvorado.yaml` in `config/` as well
- 💥 *inlet*: unknown interfaces are not skipped anymore
- ✨ *console*: add subnet aggregation for `SrcAddr` and `DstAddr`
- ✨ *inlet*: expose `Interface.Index` and `Interface.VLAN` to interface classification
- ✨ *inlet*: add `Reject()` to the set of classification functions to drop the current flow
- ✨ *inlet*: add `SetName()` and `SetDescription()` to modify interface name and description during classification
- ✨ *inlet*: add `Format()` to format a string during classification
- 🩹 *inlet*: fix parsing of sFlow containing IPv4/IPv6 headers
- 🌱 *orchestrator*: accept an `!include` tag to include other YAML files in `akvorado.yaml`

## 1.7.2 - 2023-02-12

When upgrading to this release, it takes some time to reduce the storage size
for a few columns.

- ✨ *console*: add “%” to available units
- 🩹 *inlet*: fix parsing of sFlow IPv4/IPv6 data
- 🩹 *inlet*: fix `Bytes` value for sFlow (this is the L3 length)
- 🩹 *orchestrator*: fix disabling of `DstASPath`
- 🩹 *console*: fix time range selection
- 🩹 *console*: fix calculation of the L2 overhead when selecting L2 bps
- 🩹 *console*: fix behavior of dimension limit field when empty
- 🌱 *console*: accept `IN` and `NOTIN` operators for `ExporterAddr`, `SrcAddr`, `DstAddr`, `SrcAddrNAT`, `DstAddrNAT`
- 🌱 *inlet*: optimize to reduce the number of queries to the system clock
- 🌱 *orchestrator*: reduce storage for `InIfDescription`, `OutIfDescription`, `SrcAddr`, `DstAddr`, `Bytes`, and `Packets`

## 1.7.1 - 2023-01-27

This is an important bugfix release. `DstNet*` values were classified using the
source address instead of the destination address.

- 🩹 *orchestrator*: fix `DstNet*` values
- 🌱 *inlet*: if available, use sFlow for `DstASPath`
- 🌱 *docker*: update Kafka UI image

## 1.7.0 - 2023-01-26

This version introduces the ability to customize the data schema used by
*Akvorado*. This change is quite invasive and you should be cautious when
deploying it. It requires a restart of ClickHouse after upgrading the
orchestrator. It also takes some time to reduce the storage size for `SrcPort`
and `DstPort`.

The orchestrator automatically defines the TTL for the system log tables (like
`system.query_log`). The default TTL is 30 days. You can disable that by setting
`orchestrator`→`clickhouse`→`system-log-ttl` to 0.

- ✨ *inlet*: add `schema`→`enabled`, `schema`→`disabled`,
  `schema`→`main-table-only`, and `schema`→`not-main-table-only` to alter
  collected data
- ✨ *inlet*: add the following collected data (disabled by default):
  - `SrcAddrNAT` and `DstAddrNAT`
  - `SrcPortNAT` and `DstPortNAT`
  - `SrcMAC` and `DstMAC`
  - `SrcVlan` and `DstVlan`
- 🩹 *inlet*: handle correctly interfaces with high indexes for sFlow
- 🩹 *docker*: fix Kafka healthcheck
- 🌱 *inlet*: improve decoding/encoding performance (twice faster!)
- 🌱 *orchestrator*: set TTL for ClickHouse system log tables and `exporters` table
- 🌱 *orchestrator*: reduce storage size for `SrcPort` and `DstPort`
- 🌱 *orchestrator*: add `clickhouse`→`kafka`→`engine-settings` to configure additional settings for the Kafka engine
- 🌱 *common*: Go profiler endpoints are enabled by default

## 1.6.4 - 2022-12-22

There is a schema update in this version: you also have to restart ClickHouse
after upgrading for it to pick the new schema.

This version also introduces a cache for some HTTP requests, notably those to
plot the graphs in the “Visualize” tab. The default backend is in-memory,
however the shipped `akvorado.yaml` configuration file is using Redis instead.
The `docker-compose` setup has also been updated to start a Redis container for
this usage. Use of Redis is preferred but on upgrade, you need to enable it
explicitely by adding `console`→`http`→`cache` in your configuration.

- ✨ *console*: cache some costly requests to the backend
- ✨ *console*: add `SrcNetPrefix` and `DstNetPrefix` (as a dimension and a filter attribute)
- ✨ *inlet*: add `inlet`→`flow`→`inputs`→`use-src-addr-for-exporter-addr` to override exporter address
- 🌱 *console*: add `limit` and `graph-type` to `console`→`default-visualize-options` 
- 🌱 *docker*: published `docker-compose.yml` file pins Akvorado image to the associated release
- 🌱 *docker*: update Zookeeper and Kafka images (this upgrade is optional)

## 1.6.3 - 2022-11-26

- ✨ *console*: add *100% stacked* graph type
- 🩹 *inlet*: handle non-fatal BMP decoding errors more gracefully
- 🩹 *inlet*: fix a small memory leak in BMP collector
- 🩹 *console*: fix selection of the aggregate table to not get empty graphs
- 🩹 *console*: use configured dimensions limit for “Visualize” tab
- 🌱 *inlet*: optimize BMP CPU usage, memory usage, and lock times 
- 🌱 *inlet*: replace LRU cache for classifiers by a time-based cache
- 🌱 *inlet*: add TLS support for Kafka transport
- 🌱 *console*: <kbd>Ctrl-Enter</kbd> or <kbd>Cmd-Enter</kbd> when editing a filter now applies the changes
- 🌱 *console*: switch to TypeScript for the frontend code

## 1.6.2 - 2022-11-03

- ✨ *orchestrator*: add `orchestrator`→`network-sources` to fetch network attributes with HTTP
- ✨ *console*: add `console`→`database`→`saved-filters` to populate filters from the configuration file
- 🩹 *doc*: durations must be written using a suffix (like `5s`)
- 🌱 *docker*: provider a tarball with essential files to install or upgrade a `docker-compose` setup
- 🌱 *inlet*: skip unknown AFI/SAFI in BMP route monitoring messages

## 1.6.1 - 2022-10-11

- 🩹 *inlet*: fix SrcAS when receiving flows with sFlow
- 🩹 *inlet*: do not half-close BMP connection (a remote IOS XR closes its own end)
- 🌱 *docker*: split demo exporters out of `docker-compose.yml`
- 🌱 *console*: make the upper limit for dimensions configurable
  (`console`→`dimensions-limit`)

## 1.6.0 - 2022-09-30

This release features a BMP collector to grab BGP routes from one or
several routers. The routes can be used to determine source and
destination AS (instead of using GeoIP or information from the flows)
but also the AS paths and the communities. Check `inlet`→`bmp` and
`inlet`→`core` configuration settings for more information.

- ✨ *inlet*: BMP collector to get AS numbers, AS paths, and communities from BGP [PR #155][]
- ✨ *inlet*: add `inlet`→`snmp`→`agents` to override exporter IP address for SNMP queries
- 🩹 *inlet*: handle sFlow specific interface number for locally
  originated/terminated traffic, discarded traffic and traffic sent to
  multiple interfaces
- 🌱 *build*: Docker image is built using Nix instead of Alpine

[PR #155]: https://github.com/akvorado/akvorado/pull/155

## 1.5.8 - 2022-09-18

This release bumps the minimal required version for ClickHouse to
22.4. The `docker-compose` file has been updated to use ClickHouse
22.8 (which is a long term version). Moreover, *Akvorado* now has its
own organisation and the code is hosted at
[akvorado/akvorado](https://github.com/akvorado/akvorado).

- 💥 *console*: make ClickHouse interpolate missing values (ClickHouse 22.4+ is required)
- 🩹 *orchestrator*: validate configuration of other services on start
- 🩹 *inlet*: correctly parse `inlet`→`snmp`→`communities` when it is just a string
- 🌱 *cmd*: print a shorter message when an internal error happens when parsing configuration
- 🌱 *inlet*: add `inlet`→`snmp`→`ports` to configure SNMP exporter ports

## 1.5.7 - 2022-08-23

- ✨ *inlet*: add support for flow rate-limiting with `inlet`→`flow`→`rate-limit`
- 🌱 *inlet*: improve performance of GeoIP lookup
- 🌱 *inlet*: add `inlet`→`core`→`asn-providers` to specify how to get AS
  numbers. `inlet`→`core`→`ignore-asn-from-flow` is deprecated and mapped
  to `geoip`.

## 1.5.6 - 2022-08-16

- ✨ *inlet*: add support for SNMPv3 protocol
- 🌱 *inlet*: `inlet`→`snmp`→`default-community` is now deprecated
- 🌱 *console*: make “previous period” line more visible
- 🩹 *geoip*: fix `inlet`→`geoip`→`country-database` rename to `inlet`→`geoip`→`geo-database`

## 1.5.5 - 2022-08-09

- ✨ *console*: add an option to also display flows in the opposite direction on time series graph
- ✨ *console*: add an option to also display the previous period (day, week, month, year) on stacked graphs
- 🌱 *inlet*: Kafka key is now a 4-byte random value making scaling less dependent on the number of exporters
- 🌱 *demo-exporter*: add a setting to automatically generate a reverse flow
- 🌱 *docker-compose*: loosen required privileges for `conntrack-fixer`

## 1.5.4 - 2022-08-01

`SrcCountry`/`DstCountry` were incorrectly filled in aggregated
tables. This is fixed with this release, but this implies dropping the
existing data (only the country information). See [PR #61][] for more
details.

- ✨ *inlet*: `inlet`→`core`→`default-sampling-rate` also accepts a map from subnet to sampling rate
- ✨ *inlet*: `inlet`→`core`→`override-sampling-rate` enables overriding the sampling rate received from a device
- 🩹 *orchestrator*: fix `SrcCountry`/`DstCountry` columns in aggregated tables [PR #61][]
- 🌱 *inlet*: `inlet`→`geoip`→`country-database` has been renamed to `inlet`→`geoip`→`geo-database`
- 🌱 *inlet*: add counters for GeoIP database hit/miss
- 🌱 *inlet*: `inlet`→`snmp`→`communities` accepts subnets as keys
- 🌱 *docker-compose*: disable healthcheck for the conntrack-fixer container

[PR #61]: https://github.com/akvorado/akvorado/pull/61

## 1.5.3 - 2022-07-26

- 💥 *cmd*: replace the `fake-exporter` subcommand by `demo-exporter` to make easier to understand its purpose
- 🌱 *console*: make `<<` and `!<<` operators more efficient

## 1.5.2 - 2022-07-26

- ✨ *console*: add `<<`/`!<<` operator for `SrcAddr` and `DstAddr` to match on a subnet [PR #57][]
- 🩹 *build*: remove `-dirty` from version number in released Docker images
- 🌱 *console*: hide `::ffff:` prefix from IPv6-mapped IPv4 addresses

[PR #57]: https://github.com/akvorado/akvorado/pull/57

## 1.5.1 - 2022-07-22

- 🩹 *cmd*: do not merge user-provided lists with defaults when parsing configuration
- 🩹 *docker-compose*: make `docker-compose.yml` work with Docker Compose v2/v3
- 🩹 *inlet*: update UDP packet counters when receiving packets, not after decoding
- 🌱 *console*: add configuration for default options of the visualize
  tab and the top widgets to display on the home page.

## 1.5.0 - 2022-07-20

This release introduce a new protobuf schema. When using
`docker-compose`, a restart of ClickHouse is needed after upgrading
the orchestrator to load this new schema.

- ✨ *inlet*: add sflow support [PR #23][]
- ✨ *inlet*: classify exporters to group, role, site, region, and tenant [PR #14][]
- ✨ *orchestrator*: add role, site, region, and tenant attributes to networks [PR #15][]
- ✨ *docker-compose*: clean conntrack entries when inlet container starts
- 🩹 *console*: fix use of `InIfBoundary` and `OutIfBoundary` as dimensions [PR #11][]
- 🩹 *build*: make *Akvorado* compile on MacOS
- 🌱 *inlet*: ask the kernel to timestamp incoming packets
- 🌱 *orchestrator*: limit number of Kafka consumers in ClickHouse to the number of CPUs
- 🌱 *doc*: add configuration for Juniper devices
- 🌱 *docker-compose*: add [UI for Apache Kafka][] to help debug starter issues

[PR #11]: https://github.com/akvorado/akvorado/pull/11
[PR #14]: https://github.com/akvorado/akvorado/pull/14
[PR #15]: https://github.com/akvorado/akvorado/pull/15
[PR #23]: https://github.com/akvorado/akvorado/pull/23
[UI for Apache Kafka]: https://github.com/provectus/kafka-ui

## 1.4.2 - 2022-07-16

- ✨ *inlet*: add an option to ignore ASN received from flows [PR #7][]
- 🩹 *console*: fix maximum value for the grid view
- 🌱 *orchestrator*: adapt partition key for each consolidated flow
  tables in ClickHouse to limit the number of partitions (this change
  won't be applied on an existing installation)
- 🌱 *inlet*: add `default-sampling-rate` as an option
- 🌱 *inlet*: only require either input or output interface for a valid flow
- 🌱 *build*: switch from Yarn to npm as a Javascript package manager [PR #4][]
- 🌱 *docker-compose*: pull image from GitHub instead of building it
- 🌱 *doc*: add more tips to the troubleshooting section

[PR #4]: https://github.com/akvorado/akvorado/pull/4
[PR #7]: https://github.com/akvorado/akvorado/pull/7

## 1.4.1 - 2022-07-12

- 🔒 *docker-compose*: expose two HTTP endpoints, one public (8081) and one private (8080)
- 🌱 *docker-compose*: restart ClickHouse container on failure

## 1.4.0 - 2022-07-09

- 🚀 first public release under the AGPL 3.0 license
