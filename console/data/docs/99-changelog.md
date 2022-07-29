# Changelog

For each version, changes are listed in order of importance. Minor
changes are not listed here. Each change is mapped to a category
identified with a specific icon:

- 💥: breaking change
- ✨: new feature
- 🗑️: removed feature
- 🔒: security fix
- 🩹: bug fix
- 🌱: miscellaneous change

## Unreleased

`SrcCountry`/`DstCountry` were incorrectly filled in aggregated
tables. This is fixed with this release, but this implies dropping the
existing data. See [PR #61][] for more details.

- 🩹 *orchestrator*: fix `SrcCountry`/`DstCountry` columns in aggregated tables [PR #61][]
- 🌱 *inlet*: `inlet.geoip.country-database` has been renamed to `inlet.geoip.geo-database`
- 🌱 *inlet*: add counters for GeoIP database hit/miss
- 🌱 *docker-compose*: disable healthcheck for the conntrack-fixer container

[PR #61]: https://github.com/vincentbernat/akvorado/pull/61

## 1.5.3 - 2022-07-26

- 💥 *cmd*: replace the `fake-exporter` subcommand by `demo-exporter` to make easier to understand its purpose
- 🌱 *console*: make `<<` and `!<<` operators more efficient

## 1.5.2 - 2022-07-26

- ✨ *console*: add `<<`/`!<<` operator for `SrcAddr` and `DstAddr` to match on a subnet [PR #57][]
- 🩹 *build*: remove `-dirty` from version number in released Docker images
- 🌱 *console*: hide `::ffff:` prefix from IPv6-mapped IPv4 addresses

[PR #57]: https://github.com/vincentbernat/akvorado/pull/57

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

[PR #11]: https://github.com/vincentbernat/akvorado/pull/11
[PR #14]: https://github.com/vincentbernat/akvorado/pull/14
[PR #15]: https://github.com/vincentbernat/akvorado/pull/15
[PR #23]: https://github.com/vincentbernat/akvorado/pull/23
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

[PR #4]: https://github.com/vincentbernat/akvorado/pull/4
[PR #7]: https://github.com/vincentbernat/akvorado/pull/7

## 1.4.1 - 2022-07-12

- 🔒 *docker-compose*: expose two HTTP endpoints, one public (8081) and one private (8080)
- 🌱 *docker-compose*: restart ClickHouse container on failure

## 1.4.0 - 2022-07-09

- 🚀 first public release under the AGPL 3.0 license
