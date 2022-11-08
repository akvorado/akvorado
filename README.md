# Akvorado: flow collector, enricher and visualizer &middot; [![Build status](https://img.shields.io/github/workflow/status/akvorado/akvorado/CI?style=flat-square)](https://github.com/akvorado/akvorado/actions/workflows/ci.yml) [![License](https://img.shields.io/github/license/akvorado/akvorado?style=flat-square)](LICENSE.txt) [![Latest release](https://img.shields.io/github/v/release/akvorado/akvorado?style=flat-square)](https://github.com/akvorado/akvorado/releases)

This program receives flows (currently Netflow/IPFIX and sFlow), enrice them
with interface names (using SNMP), geo information (using MaxMind),
and exports them to Kafka, then ClickHouse. It also exposes a web
interface to browse the collected data.

![Timeseries graph](console/data/docs/timeseries.png)

![Sankey graph](console/data/docs/sankey.png)

*Akvorado* is developed by [Free](https://www.free.fr), a French ISP,
and is licensed under the [AGPLv3 license](LICENSE.txt).

A demo site using fake data and running the latest stable version is
available on [demo.akvorado.net](https://demo.akvorado.net). It is the
direct result of running `docker-compose up` on a fresh checkout but
port 2055 is not accessible (you cannot send you own flows). Please,
be gentle with this resource. The demo site also enables you to browse
the [documentation](https://demo.akvorado.net/docs) (which is also
available in `docs/`).

Be aware that *Akvorado* is still young and should be considered as
alpha quality. At some point, some features may change in an
inconvenient way as it is difficult to mutate ClickHouse tables while
keeping all data intact.
