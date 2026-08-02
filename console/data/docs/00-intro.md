# Introduction

*Akvorado*[^name] receives network flows, such as NetFlow/IPFIX and sFlow. It enriches
them with interface names (using SNMP), and geographic information (using
[IPinfo](https://ipinfo.io/) or MaxMind). Then, it exports them to ClickHouse via
Kafka. It also provides a web interface to explore the data.

[^name]: [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

A public instance filled with generated traffic runs on
[demo.akvorado.net](https://demo.akvorado.net). It is the best place to see what
*Akvorado* looks like before installing anything.

## Tutorials

The tutorials get your started. Start here if you have never used *Akvorado* and
want to get results quickly.

- [Explore the demo site](01-explore.md): learn to read the graphs, without
  installing anything.
- [Collect your first flows](02-collect.md): install *Akvorado* and connect your
  first router to it.

## How-to guides

The guides are recipes for a specific goal. They assume you already have
*Akvorado* running.

- [Install Akvorado](10-install.md): the deployment options, the requirements,
  and the upgrade procedure.
- [Configure your exporters](11-exporters.md): the configuration snippets for
  Cisco, Juniper, Arista, Nokia, and a few others.
- [Troubleshoot Akvorado](12-troubleshooting.md): find which component is at
  fault when the flows do not arrive.
- [Operate Akvorado](13-operating.md): keep Kafka, ClickHouse and the Docker
  Compose setup in good shape.
- [Scale Akvorado](14-scaling.md): what to tune when packets are dropped or
  flows are late.

## Reference

Reference guides are technical descriptions of the machinery and how to operate
it.

- [Configuration](50-configuration.md): every configuration settings.
- [Command line and endpoints](51-usage.md): the subcommands and the HTTP
  endpoints of each service.
- [Web console](52-console.md): the manual of the web interface and the filter
  language.
- [Metrics](53-metrics.md): every metric exported for monitoring.

## Explanation

The following guides are for when you want to understand what happens behind the
scenes.

- [How Akvorado works](80-design.md): the four services, the path of a flow, and
  how the data is stored.
- [Internal design](81-internals.md): how the code is organized and why.

The [changelog](99-changelog.md) lists the changes of each version. Read it
before every upgrade.

## Getting help

> [!IMPORTANT]
> Please, do not open an issue or start a discussion unless you have read the
> various chapters of the documentation, notably the [troubleshooting
> guide](12-troubleshooting.md).

Questions go to the [discussions on
GitHub](https://github.com/akvorado/akvorado/discussions/categories/q-a).
Explain what you tried and what you observed, and include the output of the
commands from the troubleshooting guide.
