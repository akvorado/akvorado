# Introduction

*Akvorado*[^name] receives flows (currently Netflow/IPFIX), hydrates
them with interface names (using SNMP), geo information (using
MaxMind), and exports them to Kafka, then ClickHouse. It also exposes
a web interface to browse the result.

[^name]: [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

## Quick start

A `docker-compose.yml` file is provided to quickly get started.
Once running, *Akvorado* web interface should be running on port 80
and an inlet accepting NetFlow available on port 2055.

```console
# docker-compose up
```

A few synthetic flows are generated in the background. They can be
disabled by removing the `akvorado-exporter*` services from
`docker-compose.yml` (or you can just stop them with `docker-compose
stop akvorado-exporter{1,2,3,4}`).

Take a look at the `docker-compose.yml` file if you want to setup the
GeoIP database. It requires two environment variables to fetch them
from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).

Be sure to flush the conntrack table after starting. See the
[troubleshooting section](05-troubleshooting.md#no-packets-received)
for more details. You also need to configure SNMP on your exporters to
accept requests from Akvorado.

## Big picture

![General design](design.svg)

*Akvorado* is split into three components:

- The **inlet service** receives flows from exporters. It poll each
  exporter using SNMP to get the *system name*, the *interface names*,
  *descriptions* and *speeds*. It query GeoIP databases to get the
  *country* and the *AS number*. It applies rules to classify
  exporters into *groups*. Interface rules attach to each interface a
  *boundary* (external or internal), a *network provider* and a
  *connectivity type* (PNI, IX, transit). The flow is exported to
  *Kafka*, serialized using *Protobuf*.

- The **configuration service** configures the external components. It
  creates the *Kafka topic* and configures *ClickHouse* to receive the
  flows from Kafka.

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
