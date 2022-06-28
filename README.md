# Akvorado: flow collector, hydrater and visualizer.

This program receives flows (currently Netflow/IPFIX), hydrates them
with interface names (using SNMP), geo information (using MaxMind),
and exports them to Kafka, then ClickHouse. It also exposes a web
interface to browse the result.

## Documentation

The [documentation](/docs/00-intro.md) is in the `docs/` directory.
