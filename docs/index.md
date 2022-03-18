---
hide:
  - navigation
  - toc
---

# Akvorado { style=display:none }

![](assets/akvorado.svg){ .akvorado-logo }

*Akvorado* is a flow collector, hydrater and exporter. It receives
flows, adds some data like interface names and geo information, and
exports them to Kafka. [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

<!-- The documentation is expected to be browsed inside Akvorado itself -->

The embedded HTTP server serves the following endpoints:

- [`/metrics`](/metrics){ target=http }: Prometheus metrics
- [`/version`](/version){ target=http }: *Akvorado* version
- [`/healthcheck`](/healthcheck){ target=http }: are we alive?
- [`/flows`](/flows?limit=1){ target=http }: next available flow
- [`/flow.proto`](/flow.proto){ target=http }: protocol buffers definition
- [`/grafana`](/grafana): Grafana web interface (if configured)

<iframe name="http" style="width: 100%; height: 200px; border: 0; background-color: #1111"></iframe>
