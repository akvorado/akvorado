# Integrations

*Akvorado* needs some integration with external components to be
useful. The most important one is Kafka, but it can also integrate
with ClickHouse and Grafana.

## Kafka

The Kafka component sends flows to Kafka. Its
[configuration](02-configuration.md#kafka) mostly needs a topic name and a list
of brokers. It is possible to let *Akvorado* manage the topic with the
appropriate settings (number of partitions, replication factor and
additional configuration entries). If the topic exists, *Akvorado*
won't update the number of partitions and the replication factor but
other settings will be updated.

Each time a new flow schema is needed, a different topic is used.
*Akvorado* suffixes the topic name with the version to ensure this
property.

## ClickHouse

ClickHouse can collect the data from Kafka. To help its configuration,
*Akvorado* exposes a few HTTP endpoint:

- `/api/v0/clickhouse/init.sh` contains the schemas in the form of a
  script to execute during initialization
- `/api/v0/clickhouse/protocols.csv` contains a CSV with the mapping
  between protocol numbers and names
- `/api/v0/clickhouse/asns.csv` contains a CSV with the mapping
  between AS numbers and organization names

When [configured](02-configuration.md#clickhouse), it can also populate
the database with the appropriate tables and manages them. As a
prerequisite, the script contained in `/api/v0/clickhouse/init.sh`
should be executed. It is not possible for ClickHouse to fetch the
appropriate schemas in another way.

ClickHouse clusters are currently not supported, despite being able to
configure several servers in the configuration. Several servers are in
fact managed like they are a copy of one another.

*Akvorado* also handles database migration during upgrades. When the
protobuf schema is updated, new Kafka tables should be created, as
well as the associated materialized view. Older tables should be kept
around, notably when upgrades can be rolling (some *akvorado*
instances are still running an older version).

## Grafana

No integration is currently done for Grafana, except a reverse proxy
configured in the [web section](02-configuration.md#web).
