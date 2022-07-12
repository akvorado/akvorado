# Akvorado: flow collector, hydrater and visualizer.

This program receives flows (currently Netflow/IPFIX), hydrates them
with interface names (using SNMP), geo information (using MaxMind),
and exports them to Kafka, then ClickHouse. It also exposes a web
interface to browse the collected data.

![Timeseries graph](console/data/docs/timeseries.png)

![Sankey graph](console/data/docs/sankey.png)

*Akvorado* is developed by [Free](https://www.free.fr), a French ISP,
and is licensed under the [AGPLv3 license](LICENSE.txt). The
[documentation](console/data/docs/00-intro.md) is in the `docs/`
directory.

A demo site using fake data is available on
[demo.akvorado.net](https://demo.akvorado.net). It is the direct
result of running `docker-compose up` on a fresh checkout but port
2055 is not accessible (you cannot send you own flows). Please, be
gentle with this resource.
