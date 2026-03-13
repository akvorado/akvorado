# Changelog

For each version, changes are listed in order of importance. Minor
changes are not listed here. Each change is mapped to a category
identified with a specific icon:

- 💥: breaking change
- ✨: new feature
- 🔒: security fix
- 🩹: bug fix
- 🌱: miscellaneous change

## Unreleased

- ✨ *console*: add flows/s as a new unit for visualization
- ✨ *console*: add a heatmap visualisation
- ✨ *common*: remote data sources now support CSV and plain text formats via the `parser` option
- ✨ *outlet*: remote data sources can return exporters with `skip-missing-interfaces` set to true to fallback to the next provider
- 🩹 *outlet*: fix OpenConfig model handling in gNMI provider
- 🩹 *outlet*: fix detection of gNMI model for equipments not triggering an error on nonexistent paths
- 🩹 *outlet*: fix BMP RIB corruption due to sharing of route attribute references
- 🌱 *outlet*: shard BMP RIB to reduce lock contention
- 🌱 *outlet*: map sFlow drop codes to IPFIX ForwardingStatus
- 🌱 *orchestrator*: do not materialize TTLs in ClickHouse when updating them
- 🌱 *orchestrator*: reduce overhead of the exporters view to improve ClickHouse ingest performance
- 🌱 *orchestrator*: add ClickHouse table settings (e.g. `storage_policy`) via `table-settings`

## 2.1.2 - 2026-02-24

- ✨ *inlet*: add per-exporter flow rate limiting with `rate-limit` option
- 🌱 *outlet*: bufferize BMP messages to avoid being flagged “stuck”
- 🌱 *docs*: export all metrics in [documentation](98-metrics.md)
- 🌱 *build*: build with Go 1.26

## 2.1.1 - 2026-01-17

- 🩹 *outlet*: fix crash on malformed flow packets
- 🌱 *outlet*: handle discard on Juniper devices using inline monitoring's CPID Forwarding Exception Code

## 2.1.0 - 2026-01-10

- 💥 *docker*: remove conntrack-fixer service (this requires Docker Engine v23 or more recent)
- ✨ *inlet*: add a configuration option to decapsulate received flows (IPIP, GRE, VXLAN, and SRv6 are supported)
- ✨ *outlet*: add `FlowDirection` as a new IPFIX field (can be `undefined`, `ingress`, or `egress`)
- 🩹 *console*: add escaping for quotes and double quotes in filter language
- 🌱 *console*: accept prefixes to the right of `=` and `!=` for IP addresses fields (in addition to `<<` and `!<<`)
- 🌱 *console*: accept mixing prefixes and IPs with the `IN` and `NOTIN` operators
- 🌱 *outlet*: improve error message when exporter name is empty when using SNMP
- 🌱 *outlet*: add `flow-except-default-route` as an ASN provider

## 2.0.4 - 2025-12-04

The previous release introduced a performance regression for users with many
flows from a single exporter. This is fixed in this release.

- 🩹 *docker*: restart geoip container on boot
- 🌱 *inlet*: make load-balancing algorithm for Kafka partitions configurable
  (`random` or `by-exporter`) and revert back to `random` by default (like before 2.0.3)
- 🌱 *orchestrator*: add `kafka`→`manage-topic` flag to enable or disable topic management
- 🌱 *cmd*: make `akvorado healthcheck` use an abstract Unix socket to check service liveness

## 2.0.3 - 2025-11-19

This release contains some important fixes to prevent flow corruption under heavy load.

- 💥 *config*: `skip-verify` is false by default in TLS configurations for
  ClickHouse, Kafka and remote data sources (previously, `verify` was set to
  false by default)
- 🩹 *inlet*: keep flows from one exporter into a single partition
- 🩹 *outlet*: provide additional gracetime for a worker to send to ClickHouse
- 🩹 *outlet*: prevent discarding flows on shutdown
- 🩹 *outlet*: enhance scaling up and down workers to avoid hysteresis
- 🩹 *outlet*: accept flows where interface names or descriptions are missing
- 🩹 *docker*: update Traefik to 3.6.1 (for compatibility with Docker Engine v29)
- 🌱 *common*: enable block and mutex profiling
- 🌱 *outlet*: save IPFIX decoder state to a file to prevent discarding flows on start
- 🌱 *config*: rename `verify` to `skip-verify` in TLS configurations for
  ClickHouse, Kafka and remote data sources (with inverted logic)
- 🌱 *config*: remote data sources accept a specific TLS configuration
- 🌱 *config*: gNMI metadata provider has been converted to the same TLS
  configuration than ClickHouse, Kafka and remote data sources.
- 🌱 *docker*: update Kafka to 4.1.1
- 🌱 *docker*: update Kafbat to 1.4.2

## 2.0.2 - 2025-10-29

The modification of the default value of `inlet`→`kafka`→`queue-size` should
prevent packet drops on busier setups.

- 💥 *config*: stop shipping demo exporter configurations from the orchestrator
- ✨ *inlet*: load-balance incoming UDP packets to all workers using eBPF on
  Linux (check `docker/docker-compose-local.yaml` to enable)
- 🩹 *inlet*: fix `akvorado_inlet_flow_input_udp_in_dropped_packets_total` metric
- 🩹 *console*: fix completion tooltip being obscured with Firefox
- 🌱 *inlet*: increase default `kafka`→`queue-size` value to 4096 to prevent packet drops
- 🌱 *outlet*: be more aggressive when increasing the number of workers
- 🌱 *outlet*: cap the number of workers to the number of Kafka partitions
- 🌱 *console*: add `auth`→`logout-url` and `auth`→`avatar-url` to configure
  logout and avatar URLs when not provided as headers
- 🌱 *docker*: update Vector to 0.50.0

## 2.0.1 - 2025-10-02

- 🩹 *inlet*: disable kernel timestamping on Linux kernel older than 5.1
- 🩹 *outlet*: fix gNMI metadata provider exiting too early
- 🩹 *doc*: fix documentation for SNMPv3 configuration
- 🌱 *inlet*: add support for RFC 5103 (bidirectional flows)
- 🌱 *outlet*: handle discard and multiple interfaces for expanded sFlow samples

## 2.0.0 - 2025-09-22

This release introduces a new component: the outlet. Previously, ClickHouse was
fetching data directly from Kafka. However, this required pushing the protobuf
schema using an out-of-band method. This makes cloud deployments more complex.
The inlet now pushes incoming raw flows to Kafka without decoding them. The
outlet takes them, decodes them, enriches them, and pushes them to ClickHouse.
This also reduces the likelihood of losing packets.

This change should be transparent on most setups but you are encouraged to
review the new proposed configuration in the [quickstart tarball][] and update
your own configuration to move the appropriate configuration from the inlet to
the outlet service.

As it seems a good time as any, Zookeeper is removed from the `docker compose`
setup. ClickHouse Keeper is used instead when setting up a cluster. Kafka is now
using the KRaft mode. You need to recreate the Kafka container:

```console
# docker compose down --remove-orphans
# docker compose rm --volumes kafka
# docker volume rm akvorado_akvorado-kafka
# docker compose pull
# docker compose up -d
```

The documentation has been updated, notably the troubleshooting section.

If you use the monitoring stack, note that the Docker Compose file was split
into `docker-compose-prometheus.yml` for metrics, and `docker-compose-loki.yml`
for logs. You need to update your `.env`. Also, metric scraping is now done by
Grafana Alloy instead of Prometheus andx you need to fix the ownership of the
Prometheus volume:

```console
# docker compose run --user root --entrypoint="/bin/sh -c" prometheus "chown -R nobody:nobody /prometheus"
```

- ✨ *outlet*: new service
- ✨ *orchestrator*: automatic restart of the orchestrator service on configuration change
- 💥 *inlet*: flow rate limiting feature has been removed
- 💥 *docker*: rename `docker-compose-monitoring.yml` to `docker-compose-prometheus.yml`
- 💥 *docker*: enforce a specific IPv4 subnet (in the reserved class E)
- 💥 *common*: be stricter on results returned from remote sources
- 💥 *docker*: switch to Apache Kafka 4.1 with KRaft mode
- 💥 *docker*: switch from Prometheus to Grafana Alloy for scraping metrics
- 💥 *docker*: use profiles to optionally enable Prometheus, Loki, and Grafana
  (if you were already using them, you also need to enable the profile)
- 🩹 *console*: display missing images in documentation
- 🩹 *console*: ensure main table is used when required even when there is no data
- 🩹 *console*: fix deletion of saved filters
- 🩹 *console*: fix intermittent failure when requesting previous period
- 🩹 *docker*: move healthcheck for IPinfo updater into Dockerfile to avoid
  "unhealthy" state on non-updated installations
- 🌱 *cmd*: make `akvorado version` shorter (use `-d` for full output)
- 🌱 *inlet*: improve performance of classifiers
- 🌱 *outlet*: decode IPFIX ingressPhysicalInterface and egressPhysicalInterface
- 🌱 *outlet*: improve performance of the BMP routing provider
- 🌱 *console*: submit form on Ctrl-Enter or Cmd-Enter while selecting dimensions
- 🌱 *orchestrator*: move ClickHouse database settings from `clickhouse` to `clickhousedb`
- 🌱 *build*: accept building with a not up-to-date toolchain
- 🌱 *build*: build with Go 1.25 and use bundled toolchain
- 🌱 *build*: modernize JavaScript build with Oxlint and Rolldown-Vite
- 🌱 *build*: switch from NPM to PNPM for JavaScript build and reduce dependencies
- 🌱 *config*: listen to 4739 for IPFIX on inlet service
- 🌱 *docker*: stop spawning demo exporters by default
- 🌱 *docker*: build a linux/amd64/v3 image to enable optimizations
- 🌱 *docker*: build a linux/arm/v7 image
- 🌱 *docker*: add IPv6 configuration
- 🌱 *docker*: switch from Provectus Kafka UI (unmaintained) to Kafbat UI
- 🌱 *docker*: switch to Prometheus Java Agent exporter for Kafka
- 🌱 *docker*: update ClickHouse to 25.8 (not mandatory)
- 🌱 *docker*: update Prometheus to 3.5.0
- 🌱 *docker*: update Traefik to 3.4 (not mandatory)
- 🌱 *docker*: update node-exporter to 1.9.1
- 🌱 *docker*: add Loki to the observability stack
- 🌱 *docker*: add cAdvisor to the observability stack
- 🌱 *docker*: add examples to enable authentication and TLS
- 🌱 *docker*: change default log level for ClickHouse from trace to information
- 🌱 *docker*: enable HTTP compression for Traefik
- 🌱 *docker*: enable access log for Traefik
- 🌱 *docker*: expose Kafka UI (read-only) to the public endpoint
- 🌱 *docker*: expose Traefik Dashboard (read-only) to the public endpoint
- 🌱 *docker*: expose metrics to the public endpoint
- 🌱 *documentation*: document how to tune TCP receive buffer for BMP routing provider
- 🌱 *documentation*: document how to update the database schema for installations before 1.10.0

[quickstart tarball]: https://github.com/akvorado/akvorado/releases/latest/download/docker-compose-quickstart.tar.gz

## 1.11.5 - 2025-05-11

- 💥 *console*: Firefox 128+, Safari 16.4+, or Chrome 111+ are now required
- 🩹 *inlet*: don't override flow-provided VLANs with VLAN from Ethernet header
- 🩹 *docker*: fix console not always starting because the orchestrator didn't wait for Kafka to be ready
- 🌱 *orchestrator*: put SASL parameters in their own section in Kafka configuration
- 🌱 *orchestrator*: add OAuth support to Kafka client

## 1.11.4 - 2025-04-26

- 💥 *inlet*: in SNMP metadata provider, prefer ifAlias over ifDescr for interface description
- 🌱 *inlet*: add back `geoip` as an option for `inlet`→`core`→`asn-providers`
- 🌱 *inlet*: allow the static provider to fall back to the next provider if some
  interfaces are missing, when setting the `skip-missing-interfaces` option to
  true.
- 🌱 *build*: minimum Go version to build is now 1.24
- 🌱 *build*: use PGO for better performance of the inlet
- 🌱 *orchestrator*: add ability to override ClickHouse or Kafka configuration in some components
- 🌱 *docker*: make most containers wait for their dependencies to be healthy
- 🌱 *docker*: switch from `bitnami/valkey` to `valkey/valkey`
- 🌱 *docker*: update Kafka to 3.8 (not mandatory)
- 🔒 *docker*: update Traefik to 3.3 (security issue)

## 1.11.3 - 2025-02-04

- 💥 *inlet*: in SNMP metadata provider, use ifName for interface names and
  ifDescr or ifAlias for descriptions and make description optional
- ✨ *console*: add a "Last" column in the data table
- 🔒 *docker*: do not expose the /debug endpoint on the public entrypoint
- 🩹 *docker*: configure ClickHouse to not alter default user for new installs
- 🩹 *console*: fix synchronization of saved filters from configuration file
- 🌱 *orchestrator*: sets TTL for more ClickHouse log tables (including `text_log`)
- 🌱 *inlet*: decode destination BGP communities in sFlow packets
- 🌱 *inlet*: for SNMP configuration, unify SNMPv2 and SNMPv3 credentials into a
  single `credentials` structure

## 1.11.2 - 2024-11-01

- 🩹 *inlet*: fix decoding of QinQ in Ethernet packets
- 🩹 *console*: fix ordering of top rows when multiple sampling rates are used
- 🌱 *docker*: update ClickHouse to 24.8 (not mandatory)
- 🌱 *docker*: update to Traefik 3.1 (not mandatory)
- 🌱 *docker*: add docker/docker-compose-local.yml for local overrides

## 1.11.1 - 2024-09-01

For upgrading, you should use the "upgrade tarball" instead of the "quickstart
tarball". This new tarball does not update the configuration files or the
`.env` file.

- 🩹 *console*: sort results by number of packets when unit is packets per second
- 🌱 *inlet*: use AS path from routing component when sFlow receives an empty one
- 🌱 *console*: add `bidirectional` and `previous-period` as configurable values for default visualize options
- 🌱 *docker*: build IPinfo updater image from CI
- 🌱 *docker*: update Kafka UI to 0.7.2
- 🌱 *docker*: provide an upgrade tarball in addition to the quickstart tarball
- 🌱 *build*: minimum Go version to build is now 1.22

## 1.11.0 - 2024-06-26

- 💥 *console*: persist metadata cache on the default `docker compose` setup
- 🩹 *orchestrator*: fix population of `DstNetSite` and `SrcNetSite`
- 🩹 *orchestrator*: remove previous networks.csv temporary files on start
- 🌱 *inlet*: add NetFlow V5 support
- 🌱 *console*: add support for PostgreSQL and MySQL to store filters
- 🌱 *console*: add `console`→`homepage-graph-timerange` to define the time range for the homepage graph
- 🌱 *console*: enable round-robin for ClickHouse connections
- 🌱 *console*: display TCP and UDP port names if known
- 🌱 *orchestrator*: add ClickHouse version check for INTERPOLATE bug
- 🌱 *docker*: add monitoring stack with Prometheus and Grafana (work in progress, not enabled by default, check `.env`)
- 🌱 *docker*: update to Traefik 3.0 (not mandatory)
- 🌱 *docker*: update ClickHouse to 24.3 (not mandatory)
- 🌱 *docker*: switch from Redis to Valkey (not mandatory)
- 🌱 *docker*: build IPinfo updater image to make it available for non-x86
  architectures and ensure databases are downloaded only when an update is
  available

## 1.10.2 - 2024-04-27

- 🩹 *orchestrator*: do not use AS names from GeoIP as tenant for networks
- 🩹 *inlet*: fix sampling rate parsing for IPFIX packets using "packet interval"
- 🩹 *inlet*: fix `inlet`→`metadata`→`providers`→`targets` for gNMI provider

## 1.10.1 - 2024-04-14

- 🩹 *inlet*: fix versioning of metadata cache
- 🩹 *orchestrator*: fix panic in networks CSV refresher

## 1.10.0 - 2024-04-08

In this release, geo IP is now performed in ClickHouse instead of the inlet. When
using the standard `docker compose` setup, the configuration should be
automatically migrated from the inlet component to the orchestrator component.
This also changes how geo IP is used for AS numbers: geo IP is used as a last
resort when configured. It also increases memory usage (1.3GB for ClickHouse).

Another new feature is the ability to use a ClickHouse cluster deployment. This
is enabled when specifying a cluster name in `clickhouse`→`cluster`. There is no
automatic migration of an existing database. You should start from scratch and
copy data from the previous setup. Do not try to enable cluster mode on an
existing setup!

New installations should also get better compression and performance from the
main table, due to a change in the primary key used for this table. Check this
[Altinity article][] if you want to apply the change to your installation.

Support for Docker Compose v1 (`docker-compose` command) has been removed in
favor of Docker Compose v2 (`docker compose` command). On Ubuntu/Debian systems,
this means you can no longer use the `docker-compose` package. On Ubuntu, you
can install the `docker-compose-v2` package. For other options, check the
[documentation for installing the Compose plugin][].

- 💥 *inlet*: GeoIP data is moved from inlets to ClickHouse, add city and region
- 💥 *console*: persist console database on the default `docker compose` setup
- 💥 *docker*: remove support for `docker-compose` V1
- ✨ *orchestrator*: add support for ClickHouse clusters
- ✨ *inlet*: add gNMI metadata provider
- ✨ *inlet*: static metadata provider can provide exporter and interface metadata
- ✨ *inlet*: static metadata provider can fetch its configuration from an HTTP endpoint
- ✨ *inlet*: metadata can be fetched from multiple providers (eg, static, then SNMP)
- ✨ *inlet*: add support for several SNMPv2 communities
- ✨ *inlet*: timestamps for NetFlow/IPFIX can now be retrieved from packet content, see `inlet`→`flow`→`inputs`→`timestamp-source`
- 🩹 *cmd*: fix parsing of `inlet`→`metadata`→`provider`→`ports`
- 🩹 *console*: fix use of `InIfBoundary` and `OutIfBoundary` as dimensions
- 🌱 *orchestrator*: add TLS support to connect to ClickHouse database
- 🌱 *docker*: update to Redis 7.2, Kafka 3.7, Kafka UI 0.7.1, and Zookeeper 3.8 (not mandatory)
- 🌱 *orchestrator*: improved ClickHouse schema to increase performance

[altinity article]: https://kb.altinity.com/altinity-kb-schema-design/change-order-by/
[documentation for installing the compose plugin]: https://docs.docker.com/compose/install/linux/

## 1.9.3 - 2024-01-14

- 💥 *inlet*: many metrics renamed to match [Prometheus best practices](https://prometheus.io/docs/practices/naming/)
- ✨ *inlet*: add the following collected data (disabled by default):
  `MPLSLabels`, `MPLS1stLabel`, `MPLS2ndLabel`, `MPLS3rdLabel`, and `MPLS4thLabel`
- 🩹 *inlet*: fix static metadata provider configuration validation
- 🩹 *inlet*: fix a [performance regression][] while enriching flows
- 🩹 *inlet*: do not decode L4 header if IP packet is fragmented
- 🩹 *inlet*: handle exporters using several sampling rates
- 🌱 *docker*: update ClickHouse to 23.8 (not mandatory)
- 🌱 *orchestrator*: add `orchestrator`→`clickhouse`→`prometheus-endpoint` to configure an endpoint to expose metrics to Prometheus

[performance regression]: https://github.com/akvorado/akvorado/discussions/988

## 1.9.2 - 2023-11-28

- 🩹 *docker*: ensure ClickHouse init script is executed even when the database already exists

## 1.9.1 - 2023-10-06

- 🌱 *console*: add filtering support for custom columns
- 🌱 *inlet*: update [Expr](https://expr.medv.io/), the language behind the
  classifiers: support for variables
- 🌱 *inlet*: add RFC 7133 support for IPFIX (data link frame)
- 🌱 *orchestrator*: improve performance when looking up `SrcNetPrefix` and
  `DstNetPrefix` when these columns are materialized

## 1.9.0 - 2023-08-26

- 💥 *cmd*: use `AKVORADO_CFG_` as a prefix for environment variables used to
  modify configuration (`AKVORADO_CFG_ORCHESTRATOR_HTTP_LISTEN` instead of
  `AKVORADO_ORCHESTRATOR_HTTP_LISTEN`)
- 💥 *inlet*: `inlet`→`metadata`→`provider(snmp)`→`ports` is now a map from
  exporter subnets to ports, instead of a map from agent subnets to ports. This
  is aligned with how `communities` and `security-parameters` options behave.
- ✨ *inlet*: add [IPinfo](https://ipinfo.io/) geo IP database support and use
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
- 🩹 *inlet*: fix NetFlow processing when template is received with data
- 🩹 *inlet*: use sampling rate in NetFlow data packet if available
- 🩹 *console*: fix display when using “%” units and interface speed is 0
- 🩹 *orchestrator*: create flows table with
  `allow_suspicious_low_cardinality_types` to ensure `LowCardinality(IPv6)` can be used.
- 🌱 *inlet*: update [Expr](https://expr.medv.io/), the language behind the
  classifiers: new builtins are available
- 🌱 *build*: minimum supported Node.js version is now 16
- 🌱 *docker*: move Docker-related files to `docker/`
- 🌱 *docker*: update ClickHouse to 23.3 (not mandatory)
- 🌱 *docker*: update to Zookeeper 3.8 (not mandatory)
- 🌱 *docker*: update to Kafka 3.5 (not mandatory, but there is also a configuration change)
- 🌱 *docker*: add healthchecks for Redis and Zookeeper
- 🌱 *console*: emphasize trajectory on Sankey graphs

## 1.8.3 - 2023-04-28

- 🩹 *docker*: ensure Kafka does not use KRaft by default
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

- 💥 *docker*: the configuration files are now shipped in a `config/`
  directory: you need to move your `akvorado.yaml` to `config/` as well
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

This is an important bug fix release. `DstNet*` values were classified using the
source address instead of the destination address.

- 🩹 *orchestrator*: fix `DstNet*` values
- 🌱 *inlet*: if available, use sFlow for `DstASPath`
- 🌱 *docker*: update Kafka UI image

## 1.7.0 - 2023-01-26

This version introduces the ability to customize the data schema used by
*Akvorado*. This change is quite invasive and you should be careful when
deploying it. It requires a ClickHouse restart after upgrading the
orchestrator. It also takes some time to reduce the storage size for `SrcPort`
and `DstPort`.

The orchestrator automatically defines the TTL for the system log tables (like
`system.query_log`). The default TTL is 30 days. You can disable this by setting
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
- 🌱 *orchestrator*: add `clickhouse`→`kafka`→`engine-settings` to configure additional Kafka engine settings
- 🌱 *common*: Go profiler endpoints are enabled by default

## 1.6.4 - 2022-12-22

There is a schema update in this version: you also have to restart ClickHouse
after upgrading for it to pick up the new schema.

This version also introduces a cache for some HTTP requests, notably those to
plot the graphs in the “Visualize” tab. The default backend is in-memory,
however the shipped `akvorado.yaml` configuration file is using Redis instead.
The `docker-compose` setup has also been updated to start a Redis container for
this usage. Using Redis is preferred but on upgrade, you need to enable it
explicitly by adding `console`→`http`→`cache` in your configuration.

- ✨ *console*: cache some costly requests to the backend
- ✨ *console*: add `SrcNetPrefix` and `DstNetPrefix` (as a dimension and a filter attribute)
- ✨ *inlet*: add `inlet`→`flow`→`inputs`→`use-src-addr-for-exporter-addr` to override exporter address
- 🌱 *console*: add `limit` and `graph-type` to `console`→`default-visualize-options` 
- 🌱 *docker*: published `docker-compose.yml` file pins the Akvorado image to the associated release
- 🌱 *docker*: update Zookeeper and Kafka images (upgrade is optional)

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
- 🩹 *documentation*: durations must be written using a suffix (like `5s`)
- 🌱 *docker*: provide a tarball with essential files to install or upgrade a `docker-compose` setup
- 🌱 *inlet*: skip unknown AFI/SAFI in BMP route monitoring messages

## 1.6.1 - 2022-10-11

- 🩹 *inlet*: fix SrcAS when receiving flows with sFlow
- 🩹 *inlet*: do not half-close BMP connection (remote IOS XR closes its own end)
- 🌱 *docker*: split demo exporters out of `docker-compose.yml`
- 🌱 *console*: make the upper limit for dimensions configurable
  (`console`→`dimensions-limit`)

## 1.6.0 - 2022-09-30

This release features a BMP collector to retrieve BGP routes from one or
several routers. The routes can be used to determine source and
destination AS (instead of using GeoIP or information from the flows),
as well as the AS paths and communities. Check `inlet`→`bmp` and
`inlet`→`core` configuration settings for more information.

- ✨ *inlet*: BMP collector to get AS numbers, AS paths, and communities from BGP [PR #155][]
- ✨ *inlet*: add `inlet`→`snmp`→`agents` to override exporter IP address for SNMP queries
- 🩹 *inlet*: handle sFlow specific interface number for locally
  originated/terminated traffic, discarded traffic and traffic sent to
  multiple interfaces
- 🌱 *build*: Docker image is built using Nix instead of Alpine

[PR #155]: https://github.com/akvorado/akvorado/pull/155

## 1.5.8 - 2022-09-18

This release bumps the minimum required version for ClickHouse to
22.4. The `docker-compose` file has been updated to use ClickHouse
22.8 (which is a long-term version). Moreover, *Akvorado* now has its
own organization and the code is hosted at
[akvorado/akvorado](https://github.com/akvorado/akvorado).

- 💥 *console*: make ClickHouse interpolate missing values (ClickHouse 22.4+ is required)
- 🩹 *orchestrator*: validate configuration of other services on start
- 🩹 *inlet*: correctly parse `inlet`→`snmp`→`communities` when it is just a string
- 🌱 *cmd*: print a shorter message when an internal error happens when parsing configuration
- 🌱 *inlet*: add `inlet`→`snmp`→`ports` to configure SNMP exporter ports

## 1.5.7 - 2022-08-23

- ✨ *inlet*: add support for flow rate-limiting with `inlet`→`flow`→`rate-limit`
- 🌱 *inlet*: improve performance of GeoIP lookup
- 🌱 *inlet*: add `inlet`→`core`→`asn-providers` to specify how to retrieve AS
  numbers. `inlet`→`core`→`ignore-asn-from-flow` is deprecated and mapped
  to `geoip`.

## 1.5.6 - 2022-08-16

- ✨ *inlet*: add SNMPv3 protocol support
- 🌱 *inlet*: `inlet`→`snmp`→`default-community` is now deprecated
- 🌱 *console*: make “previous period” line more visible
- 🩹 *geoip*: fix `inlet`→`geoip`→`country-database` rename to `inlet`→`geoip`→`geo-database`

## 1.5.5 - 2022-08-09

- ✨ *console*: add an option to also display flows in the opposite direction on time series graph
- ✨ *console*: add an option to also display the previous period (day, week, month, year) on stacked graphs
- 🌱 *inlet*: Kafka key is now a 4-byte random value making scaling less dependent on the number of exporters
- 🌱 *demo-exporter*: add a setting to automatically generate a reverse flow
- 🌱 *docker*: loosen required privileges for `conntrack-fixer`

## 1.5.4 - 2022-08-01

`SrcCountry`/`DstCountry` were incorrectly filled in aggregated
tables. This is fixed with this release, but this requires dropping the
existing data (only the country information). See [PR #61][] for more
details.

- ✨ *inlet*: `inlet`→`core`→`default-sampling-rate` also accepts a map from subnet to sampling rate
- ✨ *inlet*: `inlet`→`core`→`override-sampling-rate` enables overriding the sampling rate received from a device
- 🩹 *orchestrator*: fix `SrcCountry`/`DstCountry` columns in aggregated tables [PR #61][]
- 🌱 *inlet*: `inlet`→`geoip`→`country-database` has been renamed to `inlet`→`geoip`→`geo-database`
- 🌱 *inlet*: add counters for GeoIP database hit/miss
- 🌱 *inlet*: `inlet`→`snmp`→`communities` accepts subnets as keys
- 🌱 *docker*: disable healthcheck for the conntrack-fixer container

[PR #61]: https://github.com/akvorado/akvorado/pull/61

## 1.5.3 - 2022-07-26

- 💥 *cmd*: replace the `fake-exporter` subcommand with `demo-exporter` to make its purpose easier to understand
- 🌱 *console*: make `<<` and `!<<` operators more efficient

## 1.5.2 - 2022-07-26

- ✨ *console*: add `<<`/`!<<` operator for `SrcAddr` and `DstAddr` to match on a subnet [PR #57][]
- 🩹 *build*: remove `-dirty` from version number in released Docker images
- 🌱 *console*: hide `::ffff:` prefix from IPv6-mapped IPv4 addresses

[PR #57]: https://github.com/akvorado/akvorado/pull/57

## 1.5.1 - 2022-07-22

- 🩹 *cmd*: do not merge user-provided lists with defaults when parsing configuration
- 🩹 *docker*: make `docker-compose.yml` work with Docker Compose v2
- 🩹 *inlet*: update UDP packet counters when receiving packets, not after decoding
- 🌱 *console*: add configuration for default options of the visualize
  tab and the top widgets to display on the home page.

## 1.5.0 - 2022-07-20

This release introduces a new protobuf schema. When using
`docker-compose`, a ClickHouse restart is needed after upgrading
the orchestrator to load this new schema.

- ✨ *inlet*: add sFlow support [PR #23][]
- ✨ *inlet*: classify exporters to group, role, site, region, and tenant [PR #14][]
- ✨ *orchestrator*: add role, site, region, and tenant attributes to networks [PR #15][]
- ✨ *docker*: clean conntrack entries when inlet container starts
- 🩹 *console*: fix use of `InIfBoundary` and `OutIfBoundary` as dimensions [PR #11][]
- 🩹 *build*: make *Akvorado* compile on macOS
- 🌱 *inlet*: ask the kernel to timestamp incoming packets
- 🌱 *orchestrator*: limit the number of Kafka consumers in ClickHouse to the number of CPUs
- 🌱 *documentation*: add configuration for Juniper devices
- 🌱 *docker*: add [UI for Apache Kafka][] to help debug startup issues

[PR #11]: https://github.com/akvorado/akvorado/pull/11
[PR #14]: https://github.com/akvorado/akvorado/pull/14
[PR #15]: https://github.com/akvorado/akvorado/pull/15
[PR #23]: https://github.com/akvorado/akvorado/pull/23
[UI for Apache Kafka]: https://github.com/provectus/kafka-ui

## 1.4.2 - 2022-07-16

- ✨ *inlet*: add an option to ignore ASN received from flows [PR #7][]
- 🩹 *console*: fix maximum value for the grid view
- 🌱 *orchestrator*: adapt partition key for each consolidated flow
  table in ClickHouse to limit the number of partitions (this change
  will not be applied to existing installations)
- 🌱 *inlet*: add `default-sampling-rate` as an option
- 🌱 *inlet*: only require either input or output interface for a valid flow
- 🌱 *build*: switch from Yarn to npm as a JavaScript package manager [PR #4][]
- 🌱 *docker*: pull image from GitHub instead of building it
- 🌱 *documentation*: add more tips to the troubleshooting section

[PR #4]: https://github.com/akvorado/akvorado/pull/4
[PR #7]: https://github.com/akvorado/akvorado/pull/7

## 1.4.1 - 2022-07-12

- 🔒 *docker*: expose two HTTP endpoints, one public (8081) and one private (8080)
- 🌱 *docker*: restart ClickHouse container on failure

## 1.4.0 - 2022-07-09

- 🚀 first public release under the AGPL 3.0 license
