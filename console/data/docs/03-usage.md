# Usage

*Akvorado* uses a subcommand system. Each subcommand has its own set of options.
You can get help with `akvorado --help`. Start each service with the matching
subcommand. When started from a TTY, a service displays logs in a special
format. Without a TTY, logs are formatted as JSON.

## Common options

Each service accepts a set of common options as flags.

The `--check` option checks if the provided configuration is correct and then
stops. The `--dump` option dumps the parsed configuration with the default
values. Combine it with `--check` if you do not want the service to start.

Each service requires either a configuration file (in YAML format) or a URL to
fetch its configuration (in JSON format) as an argument.
See the [configuration section](02-configuration.md) for more information.

Only the orchestrator service should get a configuration file. The other
services should point to it.

```console
$ akvorado orchestrator /etc/akvorado/config.yaml
$ akvorado inlet http://orchestrator:8080
$ akvorado outlet http://orchestrator:8080
$ akvorado console http://orchestrator:8080
$ akvorado console http://orchestrator:8080#2
```

Each service has an HTTP server that exposes a few endpoints. All services
expose these endpoints in addition to the service-specific endpoints:

- `/api/v0/metrics`: Prometheus metrics
- `/api/v0/version`: *Akvorado* version
- `/api/v0/healthcheck`: are we alive?

Each endpoint is also exposed under the service namespace. The idea is to
expose a unified API for all services under a single endpoint with an HTTP
proxy. For example, the `inlet` service also exposes its metrics under
`/api/v0/inlet/metrics` and the `outlet` service exposes its metrics under
`/api/v0/outlet/metrics`.

## Inlet service

`akvorado inlet` starts the inlet service. It receives NetFlow/IPFIX/sFlow
packets and sends them to Kafka. The inlet service does not expose any
service-specific HTTP endpoints.

## Outlet service

`akvorado outlet` starts the outlet service. It takes flows from Kafka,
parses them, adds metadata and routing information, and sends them to
ClickHouse. The HTTP component in the service exposes these endpoints:

- `/api/v0/outlet/flows`: streams the received flows. Use this for debugging
  only, as it has a performance impact.

## Orchestrator service

`akvorado orchestrator` starts the orchestrator service. It runs as a service
and exposes an HTTP service for other components (internal and external) to
configure themselves. The Kafka topic is configured at startup and does not
need the service to be running.

These endpoints are exposed to configure other internal services:

- `/api/v0/orchestrator/configuration/inlet`
- `/api/v0/orchestrator/configuration/outlet`
- `/api/v0/orchestrator/configuration/console`

These endpoints are exposed for ClickHouse to use:

- `/api/v0/orchestrator/clickhouse/protocols.csv` contains a CSV with the mapping
  between protocol numbers and names
- `/api/v0/orchestrator/clickhouse/asns.csv` contains a CSV with the mapping
  between AS numbers and organization names

ClickHouse clusters are not currently supported, but you can configure several
servers in the configuration. Several servers are managed as if they are copies
of each other.

*Akvorado* also handles database migration during upgrades. When the protobuf
schema is updated, new Kafka tables and the associated materialized view should
be created. Older tables should be kept, especially during rolling upgrades
when some *akvorado* instances are still running an older version.

## Console service

`akvorado console` starts the console service. It provides a web console.

### Home page

![Home page](home.png)

The home page contains these statistics:

- number of flows received per second
- number of exporters
- flow distribution by AS, ports, protocols, countries, and IP families
- last flow received

### Visualize page

The most interesting page is the “visualize” tab, which allows you to explore
data with graphs.

![Timeseries graph](timeseries.png)

The collapsible panel on the left has several options to change the graph's
appearance.

- The unit for the Y-axis: layer-3 bits per second, layer-2 bits per second
  (should match interface counters), packets per second, or percentage of input
  or output interface usage. For percentage usage, you should group by exporter
  name and interface name or description for the data to be meaningful.
  Otherwise, you will get an average over the matched interfaces. Also, because
  interface speeds are retrieved infrequently, the percentage may be temporarily
  incorrect when an interface's speed changes.

- Four graph types are available: “stacked”, “lines”, and “grid” to
  display time series, and “sankey” to show flow distributions between various
  dimensions.

- For “stacked”, “lines”, and “grid” graphs, the *bidirectional*
  option adds flows in the opposite direction to the graph. They
  are displayed as negative values on the graph.

- For “stacked” graphs, the *previous period* option adds a line for
  the traffic levels from the previous period. Depending on
  the current period, the previous period can be the previous hour,
  day, week, month, or year.

- You can set the time range from a list of presets or by using
  natural language. [SugarJS](https://sugarjs.com/dates/#/Parsing) is used for
  parsing and provides examples of what is possible. Alternatively, you can
  look at the presets. You can also enter dates in ISO format, for example:
  `2022-05-22 12:33`.

- You can select a set of dimensions. For time series, dimensions are
  converted to series. They are stacked with “stacked”, displayed as simple
  lines with “lines”, and displayed in a grid with “grid”. The grid
  representation is useful if you need to compare the volume of each dimension.
  For sankey graphs, dimensions are converted to nodes. In this case, you need
  to select at least two dimensions.

- Akvorado only retrieves a limited number of series. The "limit"
  parameter defines how many. The remaining values are categorized as "Other".

- The `limitType` parameter, used with the `limit` parameter, helps find
  traffic surges in 2 modes:
  - `avg`: default mode, the query gets the highest cumulative traffic over the
    selected time.
  - `max`: the query gets the traffic bursts over the selected time.
  - `last`: the query gets the most recent (last) traffic over the selected
    time.

- The filter box contains an SQL-like expression to limit the data that is
  graphed. It has an auto-completion system that you can trigger with
  `Ctrl-Space`. `Ctrl-Enter` executes the request. You can save filters by
  providing a description. A filter can be shared with other users.

The URL contains the encoded parameters and can be shared with
others. However, the stability of the options is not currently
guaranteed, so a URL may stop working after a few upgrades.

![Sankey graph](sankey.png)

### Filter language

The filter language is similar to SQL with a few variations. Fields
listed as dimensions can usually be used. The accepted operators are `=`,
`!=`, `<`, `<=`, `>`, `>=`, `IN`, `NOTIN`, `LIKE`, `UNLIKE`, `ILIKE`,
`IUNLIKE`, `<<`, and `!<<`, when they are applicable. Here are
a few examples:

- `InIfBoundary = external` only selects flows where the incoming
  interface was classified as external. The value should not be
  quoted.
- `InIfConnectivity = "ix"` selects flows where the incoming interface is
  connected to an IX.
- `SrcAS = AS12322`, `SrcAS = 12322`, or `SrcAS IN (12322, 29447)`
  limits the source AS number of the selected flows.
- `SrcAddr = 203.0.113.4` only selects flows with the specified
  address. Note that filtering on IP addresses is usually slower.
- `SrcAddr << 203.0.113.0/24` only selects flows that match the
  specified subnet.
- `ExporterName LIKE th2-%` selects flows from routers
  that start with `th2-`.
- `ASPath = AS1299` selects flows where the AS path contains 1299.

Field names are case-insensitive. You can also add comments with
`--` for single-line comments or by enclosing them in `/*` and `*/`.

The final SQL query sent to ClickHouse is logged in the console after a
successful request. Note that using the following fields will prevent the use of
aggregated data and will therefore be slower:

- `SrcAddr` and `DstAddr`,
- `SrcPort` and `DstPort`,
- `DstASPath`,
- `DstCommunities`.

## Demo exporter service

The demo exporter service simulates a NetFlow exporter, a simple SNMP agent, and
a BMP exporter.

## Other commands

- `akvorado version` displays the version.
