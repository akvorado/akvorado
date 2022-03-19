---
hide:
  - navigation
  - toc
---

# Akvorado { style=display:none }

![](assets/images/akvorado.svg){ .akvorado-logo }

*Akvorado* is a flow collector, hydrater and exporter. It receives
flows, adds some data like interface names and geo information, and
exports them to Kafka. [Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

<!-- The documentation is expected to be browsed inside Akvorado itself -->

The embedded HTTP server serves the following endpoints:

- [`/api/v0/metrics`](/api/v0/metrics){ target=http }: Prometheus metrics
- [`/api/v0/version`](/api/v0/version){ target=http }: *Akvorado* version
- [`/api/v0/healthcheck`](/api/v0/healthcheck){ target=http }: are we alive?
- [`/api/v0/flows`](/api/v0/flows?limit=1){ target=http }: next available flow
- [`/api/v0/flow.proto`](/api/v0/flow.proto){ target=http }: protocol buffers definition
- [`/api/v0/grafana`](/api/v0/grafana): Grafana web interface (if configured)

<iframe name="http" style="width: 100%; height: 200px; border: 0; background-color: #1111"></iframe>
