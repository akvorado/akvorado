# Operate Akvorado

*Akvorado* itself needs little attention once it runs. Kafka and ClickHouse need
more. This page explains how to keep an eye on them and how to adapt the Docker
Compose setup to your needs.

If a component does not work at all, start with the [troubleshooting
guide](12-troubleshooting.md). If it works but cannot keep up with the traffic,
see the [scaling guide](14-scaling.md).

## Kafka

When you use `docker compose`, a Kafka UI runs at
`http://127.0.0.1:8080/kafka-ui/`. It provides various operational
metrics that you can check, such as the space used by each topic.

It is also served on the public port, 8081, without authentication. The same
goes for the Traefik dashboard. Before you expose your server, uncomment the
matching block in `docker/docker-compose-local.yml` to keep both on the private
port.

## ClickHouse

While ClickHouse works well out-of-the-box, we still recommend that you read
[its documentation](https://clickhouse.com/docs/).
Altinity also provides a [knowledge base](https://kb.altinity.com/)
with other tips.

> [!TIP]
> To connect to the ClickHouse database in the Docker Compose setup, use `docker
> compose exec clickhouse clickhouse-client`.

### Memory usage

The `networks` dictionary can use a lot of memory. You can check with these queries:

```sql
SELECT name, status, type, formatReadableSize(bytes_allocated)
FROM system.dictionaries
```

Moreover, ClickHouse is tuned for 32 GB of RAM or more. The ClickHouse documentation
has some tips to [run with 16 GB or
less](https://clickhouse.com/docs/operations/tips#using-less-than-16gb-of-ram).

### Space usage

To get the space used by ClickHouse, use this query:

```sql
SELECT formatReadableSize(sum(bytes_on_disk)) AS size
FROM system.parts
```

You can get an idea of how much space is used by each table with this
query:

```sql
SELECT table, formatReadableSize(sum(bytes_on_disk)) AS size, MIN(partition_id) AS oldest
FROM system.parts
WHERE table LIKE 'flow%'
GROUP by table
ORDER by sum(bytes_on_disk) DESC
```

This query shows how much space is used by each column for the `flows`
table and how much they are compressed. This can be helpful if you find that this
table uses too much space.

```sql
SELECT
    database,
    table,
    column,
    type,
    sum(rows) AS rows,
    sum(column_data_compressed_bytes) AS compressed_bytes,
    formatReadableSize(compressed_bytes) AS compressed,
    formatReadableSize(sum(column_data_uncompressed_bytes)) AS uncompressed,
    sum(column_data_uncompressed_bytes) / compressed_bytes AS ratio,
    any(compression_codec) AS codec
FROM system.parts_columns AS pc
LEFT JOIN system.columns AS c ON (pc.database = c.database) AND (c.table = pc.table) AND (c.name = pc.column)
WHERE table = 'flows' AND active
GROUP BY
    database,
    table,
    column,
    type
ORDER BY
    database ASC,
    table ASC,
    sum(column_data_compressed_bytes) DESC
```

You can reduce the space used by the `flows` table by setting a lower TTL in
`clickhouse`→`resolutions`. This does not take effect immediately. You need to
run `ALTER TABLE flows MATERIALIZE TTL`.

You can also include the system tables:

```sql
SELECT database, table, formatReadableSize(sum(bytes_on_disk)) AS size, MIN(partition_id) AS oldest
FROM system.parts
GROUP by database, table
ORDER by sum(bytes_on_disk) DESC
```

ClickHouse is configured to log various events in MergeTree tables. By
default, these tables are unbounded. Unless you configure it otherwise, the
orchestrator sets a TTL of 30 days. You can also customize these tables in the
configuration files or disable them completely. See the [ClickHouse
documentation](https://clickhouse.com/docs/en/operations/system-tables/) for
more details.

All the system tables with the suffix `_0` or `_1` are tables from an older version of
ClickHouse. You can drop them by using this SQL query and copying and pasting the
result:

```sql
SELECT concat('DROP TABLE IF EXISTS system.', name, ';')
FROM system.tables
WHERE (database = 'system') AND match(name, '_[0-9]+$')
FORMAT TSVRaw
```

### CPU usage

If ClickHouse has high CPU usage, you can find slow queries with:

```sql
SELECT formatReadableTimeDelta(query_duration_ms/1000) AS duration, query
FROM system.query_log
WHERE query_kind = 'Select'
ORDER BY query_duration_ms DESC
LIMIT 10
FORMAT Vertical
```

Also, check for slow inserts:

```sql
SELECT formatReadableTimeDelta(query_duration_ms/1000) AS duration, query
FROM system.query_log
WHERE query_kind = 'Insert'
ORDER BY query_duration_ms DESC
LIMIT 10
FORMAT Vertical
```

[Altinity's knowledge
base](https://kb.altinity.com/altinity-kb-useful-queries/query_log/)
contains other useful queries.

### Old tables

Tables that are not used anymore may still exist. Check with `SHOW TABLES`. You can
drop these tables:

- `flows_raw_errors`
- `flows_raw_errors_consumer`
- any `flows_XXXXXXX_raw_errors`
- any `flows_XXXXXXX_raw` and `flows_XXXXXXX_raw_consumer` when `XXXXXXX` does not end with `vN` where `N` is a number
- any `flows_XXXXXvN_raw` and `flows_XXXXXvN_raw_consumer` when another table exists with a higher `N` value

These tables do not contain data. If you make a mistake, you can restart the orchestrator to recreate them.

### Update the database schema

In version 1.10.0, the primary key of the `flows` table was changed to improve
performance. This update is not automatically applied to existing installations
because it requires copying data. You can check if your schema needs to be
updated with this SQL command:

```sql
SELECT primary_key
FROM system.tables
WHERE (name = 'flows') AND (database = currentDatabase())
```

If the primary key is not `toStartOfFiveMinutes(TimeReceived)`, you are using
the old schema. You may get better performance by switching to the new one.

The idea is to create a new table and transfer the data from the old table,
partition by partition. Execute this request and make sure you have enough
space to store the largest partition:

```sql
SELECT
    partition,
    formatReadableSize(sum(bytes_on_disk)) AS size,
    count() AS count
FROM system.parts
WHERE (database = currentDatabase()) AND (`table` = 'flows') AND active
GROUP BY partition
ORDER BY partition ASC
```

> [!IMPORTANT]
> There is a risk of data loss if something goes wrong. Back up your data if it
> is important to you. This guide only covers the non-clustered scenario.

#### Preparation

You need to stop the **outlet** service to make sure that nothing is writing to
ClickHouse while the migration is in progress. Get the current parameters for
the `flows` table:

```sql
SELECT engine_full
FROM system.tables
WHERE (database = currentDatabase()) AND (`table` = 'flows')
FORMAT TSVRaw
```

You need to change the `ORDER BY` directive to replace `TimeReceived` with
`toStartOfFiveMinutes(TimeReceived)` and add
`toStartOfFiveMinutes(TimeReceived)` as the primary key. You should get
something like this:

```
MergeTree PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, toIntervalSecond(25920))) PRIMARY KEY (toStartOfFiveMinutes(TimeReceived)) ORDER BY (toStartOfFiveMinutes(TimeReceived), ExporterAddress, InIfName, OutIfName) TTL TimeReceived + toIntervalSecond(1296000) SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
```

Also, check the current number of flows stored in ClickHouse:

```sql
SELECT count(*)
FROM flows
```

#### Rename the old table

Rename the current `flows` table to `flows_old`:

```sql
RENAME TABLE flows TO flows_old
```

#### Create the new table

Allow suspicious low cardinality types:

```sql
SET allow_suspicious_low_cardinality_types = true
```

Create the new `flows` table with the updated `ORDER BY` directive. After
`ENGINE = `, copy and paste the engine definition that you prepared earlier:

```sql
CREATE TABLE flows AS flows_old
ENGINE =
```

#### Create an intermediate table

Create an intermediate table to copy data to. This is needed to avoid duplicating
data in the aggregated tables. Use the same engine definition as before:

```sql
CREATE TABLE flows_temp AS flows_old
ENGINE =
```

#### Generate the migration statements

Use this SQL query to create the migration:

```sql
SELECT
 concat('insert into flows_temp select * from flows_old where _partition_id = \'', partition_id, '\';\n',
        'alter table flows_old drop partition \'', partition_id, '\';\n', 
        'alter table flows attach partition id \'', partition, '\' from flows_temp;') AS cmd
FROM system.parts
WHERE (database = currentDatabase()) AND (`table` = 'flows_old')
GROUP BY
    database,
    `table`,
    partition_id,
    partition
ORDER BY partition_id ASC
FORMAT TSVRaw
```

#### Execute the migration statements

You can execute them one by one. You can check that you still have all the flows
after each `attach partition` directive:

```sql
SELECT (
        SELECT count(*)
        FROM flows
    ) + (
        SELECT count(*)
        FROM flows_old
    )
```

#### Drop the old table

The last step is to remove the empty `flows_old` table and the intermediate
table:

```sql
DROP TABLE flows_old;
DROP TABLE flows_temp;
```

Then, you can restart the **outlet** service.

## Docker Compose

The default Docker Compose setup is meant to help you get started quickly. However,
you can also use it for a production setup.

You are allowed to modify `.env` and `docker/docker-compose-local.yml`, as well
as anything in `config/`. Everything else will be erased during upgrades.

The `.env` file tailors the complete Docker Compose setup. Some parts are
enabled using [Docker Compose
profiles](https://docs.docker.com/compose/how-tos/profiles/). You can
temporarily enable them with `--profile` flag to `docker compose`. In this case,
any profile set with `COMPOSE_PROFILES` are overridden. Some other parts require
you to uncomment additional Docker Compose configuration files directly in
`.env`.

This `docker/docker-compose-local.yml` file can override parts of the
configuration. The [merge
rules](https://docs.docker.com/reference/compose-file/merge/) are a bit complex.
The general rule is that scalars are replaced, while lists and mappings are
merged. However, there are exceptions.

> [!TIP]
> Always check if the final configuration matches your expectations with `docker compose config`.

You can disable some services by using profiles:

```yaml
services:
  akvorado-inlet:
    profiles: [ disabled ]
```

It is possible to remove a value with the `!reset` tag:

```yaml
services:
  akvorado-console:
    environment:
      AKVORADO_CFG_CONSOLE_DATABASE_DSN: !reset null
```

With Docker Compose v2.24.4 or later, you can override a value:

```yaml
services:
  traefik:
    ports: !override
      - 127.0.0.1:8080:8080/tcp
      - 80:8081/tcp
```

The `docker/docker-compose-local.yml` file contains more examples that you can
adapt to your needs. You can also enable TLS by uncommenting the appropriate
section in `.env`.

### Networking

The default setup has both IPv4 and IPv6 enabled, using the NAT setup.
For IPv6 to work correctly, you need either Docker Engine v27 or to set
`ip6tables` to `true` in `/etc/docker/daemon.json`.

If you prefer to keep the default Docker configuration, you can add this snippet
to `docker/docker-compose-local.yml`:

```yaml
networks: !reset {}
```
