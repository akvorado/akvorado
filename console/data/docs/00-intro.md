# Introduction

*Akvorado*[^name] is a flow collector, hydrater and exporter. It
receives flows, adds some data like interface names and countries, and
exports them to Kafka.

[^name]: [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

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
Kafka topic is used. For example, the `flows-v1` topic receive
serialized flows using the first version of the schema. The inlet
service exports the schemas as well as the current version with its
HTTP service, via the `/api/v0/inlet/schemas.json` endpoint.

## ClickHouse database schemas

Flows are stored in a ClickHouse database using a single table
`flows`. The configuration service keeps the table schema up-to-date.
You can check the schema using `SHOW CREATE TABLE flows`.
