# Integrations

*Akvorado* needs some integration with external components to be
useful. The most important one is Kafka, but it can also integrate
with Clickhouse and Grafana.

## Kafka

The Kafka component sends flows to Kafka. Its
[configuration](configuration.md#kafka) mostly needs a topic name and a list
of brokers. It is possible to let *Akvorado* manage the topic with the
appropriate settings (number of partitions, replication factor and
additional configuration entries). If the topic exists, *Akvorado*
won't update the number of partitions and the replication factor but
other settings will be updated.

## Clickhouse

Clickhouse can collect the data from Kafka. To help its configuration,
*Akvorado* exposes a few HTTP endpoint:

- `/api/v0/clickhouse/flow.proto` contains the schema
- `/api/v0/clickhouse/init.sh` contains the schema in the form of a
  script to execute during initialization
- `/api/v0/clickhouse/protocols.csv` contains a CSV with the mapping
  between protocol numbers and names
- `/api/v0/clickhouse/asns.csv` contains a CSV with the mapping
  between AS numbers and organization names

## Grafana

No integration is currently done for Grafana, except a reverse proxy
configured in the [web section](configuration.md#web).
