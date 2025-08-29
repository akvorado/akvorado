# Introduction

*Akvorado*[^name] receives network flows, such as NetFlow/IPFIX and sFlow. It enriches
them with interface names (using SNMP), and geographic information (using
[IPinfo](https://ipinfo.io/) or MaxMind). Then, it exports them to ClickHouse via
Kafka. It also provides a web interface to explore the data.

[^name]: [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

## Requirements

The recommended configuration is:

- 8 vCPUs (AMD64 or ARM64)
- 100 GB of disk
- 16 GB of RAM

## Quick start

The easiest way to get started is with
[Docker](https://docs.docker.com/get-docker) and [Docker Compose
V2](https://docs.docker.com/compose/install/). On Ubuntu, you can
install the `docker-compose-v2` package. On macOS, you can use the
`docker-compose` formula from Homebrew.

```console
# mkdir akvorado
# cd akvorado
# curl -sL https://github.com/akvorado/akvorado/releases/latest/download/docker-compose-quickstart.tar.gz | tar zxvf -
# docker compose up
```

Monitor the output of `docker compose ps` to see the status of the services.
Once the `akvorado-console` service is "healthy", the *Akvorado* web interface
should be running on port 8081. This can take a few minutes.

### Next steps

To connect your own network devices:

1. Customize the configuration in `akvorado.yaml`:
   - Set SNMP communities for your devices in `outlet` → `metadata` → `provider` → `communities`
   - Configure interface classification rules in `outlet` → `core` → `interface-classifiers`

1. Configure your routers/switches to send flows to *Akvorado*:
   - NetFlow: port 2055
   - IPFIX: port 4739
   - sFlow: port 6343
   
1. Restart all containers:
   - `docker compose down`
   - `docker compose up -d`

> [!TIP]
> Interface classification is essential for the web interface to work properly.
> Without it, you won't see data in the dashboard widgets or visualization tab.
> See the [configuration guide](02-configuration.md#classification) for details.

### Need help?

- Check the [installation guide](01-install.md) for other deployment options
- Read the [configuration guide](02-configuration.md) for detailed setup instructions
- Review the [operations guide](04-operations.md) for router configuration examples
- Check the [troubleshooting guide](05-troubleshooting.md) if you run into an issue

You can see the full configuration (with default values) with:
`docker compose exec akvorado-orchestrator akvorado orchestrator
--check --dump /etc/akvorado/akvorado.yaml`.

> [!IMPORTANT]
> Please, do not open an issue or start a discussion unless you have read the
> various chapters of the documentation, notably the [troubleshooting
> guide](05-troubleshooting.md).

## Big picture

![General design](design.svg)

*Akvorado* is split into four components:

- The **inlet service** receives flows from exporters and sends them to Kafka
  without parsing them.

- The **outlet service** takes flows from Kafka, parses them, and enriches them
  with metadata. It uses SNMP to poll each exporter to get the *system name*,
  *interface names*, *descriptions* and *speeds*. It applies rules to add
  attributes to exporters. Interface rules add a *boundary* (external or
  internal), a *network provider* and a *connectivity type* (PNI, IX, transit)
  to each interface. Optionally, it may also receive BGP routes through the BMP
  protocol to get the *AS number*, the *AS path*, and the communities. The
  enriched flows are then exported to ClickHouse.

- The **orchestrator service** configures the other components. It creates the
  *Kafka topic* and configures *ClickHouse* to receive the flows from the outlet
  service. It provides configuration settings for the other services. It
  provides additional data to ClickHouse, such as *GeoIP* data.

- The **console service** provides a web interface to view and analyze the flows
  in the ClickHouse database.

## ClickHouse database schemas

Flows are stored in a ClickHouse database using a table `flows` (and a
few consolidated versions). The orchestrator service keeps the table
schema up-to-date. You can check the schema using `SHOW CREATE TABLE
flows`.
