# Introduction

*Akvorado*[^name] receives flows (currently Netflow/IPFIX and sFlow), hydrates
them with interface names (using SNMP), geo information (using
MaxMind), and exports them to Kafka, then ClickHouse. It also exposes
a web interface to browse the result.

[^name]: [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

## Quick start

A `docker-compose.yml` file is provided to quickly get started. The
only requirement is to have
[Docker](https://docs.docker.com/get-docker/) and [Docker
Compose](https://docs.docker.com/compose/install/). Grab the latest
tarball from the [release page on
GitHub](https://github.com/akvorado/akvorado/releases), unpack it,
then run the `docker-compose` command:

```console
# docker-compose up
```

Once running, *Akvorado* web interface should be running on port 8080.

A few synthetic flows are generated in the background. They can be
disabled by removing the `:docker-compose-demo.yml` string from
`.env`, or by stopping them with `docker-compose stop
akvorado-exporter{1,2,3,4}`, or by removing the associated
configuration in `akvorado.yaml`.

If you want to send you own flows, the inlet is accepting both NetFlow
(port 2055) and sFlow (port 6343). You should also customize some
settings in `akvorado.yaml`. They are described in details in the
[“configuration” section](02-configuration.md) section of the
documentation.

- `clickhouse` → `asns` to give names to your internal AS numbers
- `clickhouse` → `networks` to attach attributes to your networks
- `inlet` → `snmp` → `communities` to set the communities to use for
  SNMP queries
- `inlet` → `core` → `exporter-classifiers` to define rules to attach
  attributes to your exporters
- `inlet` → `core` → `interface-classifiers` to define rules to attach
  attributes to your interfaces (including the "boundary" attribute
  which is used by default by the web interface)

You can get all the expanded configuration (with default values) with
`docker-compose exec akvorado-orchestrator akvorado orchestrator
--check --dump /etc/akvorado.yaml`.

Take a look at the `docker-compose.yml` file if you want to setup the
GeoIP database. It requires two environment variables to fetch them
from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).

## Big picture

![General design](design.svg)

*Akvorado* is split into three components:

- The **inlet service** receives flows from exporters. It poll each
  exporter using SNMP to get the *system name*, the *interface names*,
  *descriptions* and *speeds*. It query GeoIP databases to get the
  *country* and the *AS number*. It applies rules to add attributes to
  exporters. Interface rules attach to each interface a *boundary*
  (external or internal), a *network provider* and a *connectivity
  type* (PNI, IX, transit). Optionally, it may also receive BGP routes
  through the BMP protocol to get the *AS number*, the *AS path*, and
  the communities. The flow is exported to *Kafka*, serialized using
  *Protobuf*.

- The **orchestrator service** configures the internal and external
  components. It creates the *Kafka topic* and configures *ClickHouse*
  to receive the flows from Kafka. It exposes configuration settings
  for the other services to use.

- The **console service** exposes a web interface to look and
  manipulate the flows stored inside the ClickHouse database.

## Serialized flow schemas

Flows sent to Kafka are encoded with a versioned schema, described in
the `flow-*.proto` files. For each version of the schema, a different
Kafka topic is used. For example, the `flows-v2` topic receive
serialized flows using the first version of the schema. The inlet
service exports the schemas as well as the current version with its
HTTP service, via the `/api/v0/inlet/schemas.json` endpoint.

## ClickHouse database schemas

Flows are stored in a ClickHouse database using a table `flows` (and a
few consolidated versions). The orchestrator service keeps the table
schema up-to-date. You can check the schema using `SHOW CREATE TABLE
flows`.
