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

- 🩹 *console*: fix use of `InIfBoundary` and `OutIfBoundary` as dimensions [PR #11][]
- 🩹 *docker-compose*: avoid starting bogus "akvorado-image" service
- 🩹 *build*: make *Akvorado* compile on MacOS
- 🌱 *doc*: add configuration for Juniper devices
- 🌱 *docker-compose*: add [UI for Apache Kafka][] to help debug starter issues

[PR #11]: https://github.com/vincentbernat/akvorado/pull/11
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
