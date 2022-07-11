# Akvorado: flow collector, hydrater and visualizer.

This program receives flows (currently Netflow/IPFIX), hydrates them
with interface names (using SNMP), geo information (using MaxMind),
and exports them to Kafka, then ClickHouse. It also exposes a web
interface to browse the collected data.

![Timeseries graph](docs/timeseries.png)

![Sankey graph](docs/sankey.png)

*Akvorado* is developed by [Free](https://www.free.fr), a French ISP,
and is licensed under the [AGPLv3 license](LICENSE.txt). The
[documentation](docs/00-intro.md) is in the `docs/` directory.

