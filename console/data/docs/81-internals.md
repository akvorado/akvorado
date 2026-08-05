# Internal design

*Akvorado* is written in Go. Each service has its code in a separate directory
(`inlet/`, `outlet/`, `orchestrator/`, `console/`, and `demoexporter/`). The
`common/` directory contains components that are common to several services. The
`cmd/` directory contains the main entry points.

Each service is split into several components. This is heavily inspired by the
[Component framework in Clojure][]. A component is a piece of software with its
own configuration, state, and dependencies on other components.

[Component framework in Clojure]: https://github.com/stuartsierra/component

Each component has the following pieces of code:

- A `Component` structure that contains its state.
- A `Configuration` structure that contains the configuration of the
  component. It maps to a section of the [Akvorado configuration
  file](50-configuration.md).
- A `DefaultConfiguration` function with the default values for the
  configuration.
- A `New()` function that instantiates the component. This method takes
  the configuration and the dependencies. It is inert.
- Optionally, a `Start()` method to start the routines that are associated with
  the component.
- Optionally, a `Stop()` method to stop the component.

Each component is tested independently. If a component is complex, a `NewMock()`
function can create a component with a compatible interface to be used instead
of the real component. In this case, it takes a `testing.T` struct as the first
argument and starts the component immediately. It can return the real component
or a mocked version. For example, the Kafka component returns a component that
uses a mocked Kafka producer.

Dependencies are handled manually, unlike more complex component-based
solutions like [Uber Fx][].

[Uber Fx]: https://github.com/uber-go/fx

## Reporter

The reporter is a special component that handles logs and metrics for all
the other components. In the future, this could also be the place to handle
crash reports.

For logs, it is mostly a façade for
[github.com/rs/zerolog](https://github.com/rs/zerolog) with some additional
code to add the module name to the logs.

For metrics, it is a façade for the [Prometheus instrumentation
library][]. It provides a registry that automatically adds the module name to
metric names.

It also exposes a simple way to report health checks from various
components. While it could be used to kill the application
proactively, it is currently only exposed through HTTP. Not all
components have health checks. For example, for the `flow` component,
it is difficult to read from UDP while watching for a check. For the
`http` component, the health check would be too trivial (not in the
routine that handles the heavy work). For `kafka`, the hard work is hidden
by the underlying library, and we would not want to be declared
unhealthy because of a transient problem by checking broker states
manually. The `daemon` component tracks the important goroutines, so it
is not vital.

The general idea is to give good visibility to an operator.
Everything that moves should get a counter. Errors should either be
fatal or be rate-limited and counted in a metric.

[Prometheus instrumentation library]: https://github.com/prometheus/client_golang/

## CLI

The CLI (not a component) is handled by
[Cobra](https://github.com/spf13/cobra). The configuration file is
handled by [mapstructure](https://github.com/go-viper/mapstructure).
Backward compatibility is handled by registering hooks to transform the
configuration.

## Flow processing

Flow processing is split between the inlet and outlet services. Until version
2.0, a single service did everything, which forced the whole enrichment path to
be fast enough to never block the reception of UDP packets. The reasons for the
split are explained in [a blog
post](https://vincent.bernat.ch/en/blog/2025-akvorado-2.0).

### Inlet flow reception

The inlet service receives flows. The design prioritizes speed and minimal
processing. Flows are encapsulated into protobuf messages and sent to Kafka
without being parsed. The design scales by creating a socket for each worker
instead of distributing incoming flows with a channel.

On Linux, an eBPF program is attached to the `SO_REUSEPORT` group to choose the
socket for each incoming packet, using a round-robin counter. Without it, the
kernel selects the socket by hashing the packet headers, which spreads the load
badly when there are only a few exporters. Attaching the program needs the `BPF`
capability. When it fails, the input logs a warning and keeps working with the
kernel behaviour.

NetFlow v5, NetFlow v9, IPFIX, and sFlow are currently supported for reception.

The design of this component is modular. You can "plug in" new inputs
easily. Most buffering is implemented at this level by input modules that
require it. Additional buffering happens in the Kafka module.

### Outlet flow decoding

The outlet service takes flows from Kafka and performs the actual decoding
with [GoFlow2](https://github.com/NetSampler/GoFlow2). This is where flow 
parsing, enrichment with metadata and routing information, and classification 
happen before writing to ClickHouse.

A third decoder, `gob`, exists for tests only. It is excluded from release
builds with a build tag.

Each worker keeps one flow message and clears it between two flows instead of
allocating a new one. The same idea is used for the batch sent to ClickHouse.
This keeps the pressure on the garbage collector low, as the outlet handles
every single flow.

## Kafka

The Kafka component relies on [franz-go](https://github.com/twmb/franz-go). It
provides a `kfake` module that is used for most functional tests. Otherwise, if
a broker is available under the DNS name `kafka` or at `localhost` on port 9092,
it is used for a quick functional test.

This library has not been benchmarked. Previously, we used
[Sarama](https://github.com/IBM/sarama). However, the documentation is quite
poor, it relies heavily on pointers (which puts pressure on the garbage collector),
and the concurrency model is difficult to understand. Another contender could be
[kafka-go](https://github.com/segmentio/kafka-go).

## ClickHouse

For this OLAP database, migrations are done with a simple loop that
checks if a step is needed with a custom query and executes it with Go code.
Database migration systems exist in Go, notably
[migrate](https://github.com/golang-migrate/migrate), but because the
table schemas depend on the user configuration, it is preferred
to use code to check if the existing tables are up-to-date
and to update them. For example, we may want to check if the Kafka
settings of a table or the source URL of a dictionary are current.

When inserting into ClickHouse, we rely on the low-level
[ch-go](https://github.com/ClickHouse/ch-go/) library. Decoded flows are batched
directly into the wire format that is used by ClickHouse.

The console and the orchestrator build SQL queries with a Fluent-like builder
API in `common/sqlbuilder`, which outputs a syntax tree based on
[clickhouse-sql-parser](https://github.com/Aftership/clickhouse-sql-parser). The
filter language is also translated to a syntax tree and combined with the syntax
tree built by the console from the elements selected by the user. The
orchestrator also leverages the parser to compare two SQL queries and check if
an update is needed.

Functional tests are run when a ClickHouse server is available under
the name `clickhouse` or on `localhost`.

## Metadata

The metadata component turns interface indexes into names, descriptions and
speeds. It has three providers: `snmp`, `gnmi`, and `static`. SNMP polling is
done with [GoSNMP](https://github.com/gosnmp/gosnmp) and gNMI with
[gnmic](https://github.com/openconfig/gnmic).

The cache layer is tailored specifically to our needs. Cached information can
expire if it is not accessed or refreshed periodically.

If an exporter fails to answer too frequently, a circuit breaker opens for a
minute to ensure that it does not eat up all the workers' resources. The breaker
sits in the component itself, so it protects every provider.

Testing is done by another implementation of an [SNMP
agent](https://github.com/slayercat/GoSNMPServer).

## BMP

The BMP server uses [GoBGP](http://github.com/osrg/gobgp)'s
implementation. GoBGP does not have a BMP collector, but it is just a
simple TCP connection that receives BMP messages, and we use GoBGP to parse
them.

[github.com/gaissmai/bart](https://github.com/gaissmai/bart) implements a fast
trie for IP lookup with an adaptation of Knuth's ART algorithm. It is used for both
for configuring subnet-dependent settings and for storing data that is received with
BMP. In the case of BMP, we store the routes in a map that is indexed by a prefix index
(dynamically allocated, with a free list) and a route index (contiguously
allocated from 0). The tree only holds a prefix reference: the prefix index and
a generation number. The generation is bumped when an index is freed, so a
reference to a recycled index is detected instead of returning the wrong prefix.

The RIB is split into shards, 16 by default and 256 at most. Each shard has its
own lock, its own free list and its own interning pools, and its number is
encoded in the high bits of the prefix index. Several BMP connections can
therefore update the RIB at the same time. The prefix tree stays shared: writers
take a mutex and modify a copy, readers get the current tree through an atomic
pointer and never block. With tens of millions of routes, a single lock was
contended by the outlet workers, the routers sending updates, and the flush of a
peer going down. [A blog
post](https://vincent.bernat.ch/en/blog/2026-akvorado-rib-sharding) describes
the two steps of this work and the benchmarks.

To save memory, *Akvorado* "interns" next-hops, origin AS, AS paths, and
communities. Each unique combination is associated with a reference-counted
32-bit integer, which is used in the RIB instead of the original information.
Reference counting is explicit. The `unique` package of the standard library
comes with a cost: 64-bit pointer instead of 32-bit integer and concurrency
support.

## Schema

The *Akvorado* schema is a bit dynamic. You can add or remove columns of data.
However, everything needs to be predefined in the code. To add a new column, you
need to follow these steps:

1. Add its symbol to `common/schema/definition.go`.
2. Add it to the `flows()` function in `common/schema/definition.go`. Be sure to
   specify the correct/smallest ClickHouse type. If the column is prefixed with
   `Src` or `InIf`, do not add the opposite direction. This is done
   automatically. Use `ClickHouseMainOnly` if the column is expected to take up a
   lot of space. Add the column to the end and set the `Disabled` field to `true`.
   If you add several fields, create a group and use it on decoding to keep
   decoding/encoding fast for people who do not enable them.
3. Make it usable in the filters by adding it to `console/filter/parser.peg`.
   Do not forget to add a test in `console/filter/parser_test.go`.
4. Modify `console/query/column.go` to change the display of the column (it
   should be a string).
5. If it does not have a proper type in ClickHouse to be displayed as is (like a
   MAC address that is stored as a 64-bit integer), also modify
   `widgetFlowLastHandlerFunc()` in `console/widgets.go`.
6. Modify `outlet/flow/decoder/netflow/decode.go` and
   `outlet/flow/decoder/sflow/decode.go` to extract the data from the flows.
7. If it is useful, add a completion in `filterCompleteHandlerFunc()` in
   `akvorado/console/filter.go`.

## Web console

The web console is built as a REST API with a single-page application
on top of it.

### REST API

The REST API is built directly on top of `net/http`. The `common/httpserver`
package provides a small `Router` wrapper around `http.ServeMux` (using Go 1.22+
method-aware patterns) with support for route groups and middleware chains, as
well as helpers for JSON/YAML encoding, request body and query binding, and
response caching. Body binding uses the [validator
package](https://github.com/go-playground/validator), which implements value
validations based on tags. The validation options are quite rich.

### Single page application

The SPA is built with mostly these components:

- [TypeScript](https://www.typescriptlang.org) instead of JavaScript,
- [Vite](https://vitejs.dev/) as a builder,
- [Vue](https://vuejs.org/) as the reactive JavaScript framework,
- [TailwindCSS](https://tailwindcss.com/) for styling pages directly in HTML,
- [Headless UI](https://headlessui.dev/) for some unstyled UI components,
- [ECharts](https://echarts.apache.org/) to plot charts.
- [CodeMirror](https://codemirror.net/6/) to edit filter expressions.

There is no full-blown component library. When the console was written, none of
the candidates was a good fit:

- [Vuetify](https://vuetifyjs.com/) only supported Vue 2.
- [BootstrapVue](https://bootstrap-vue.org/) only supported Vue 2.
- [PrimeVue](https://www.primefaces.org/primevue/) was quite heavyweight, and many things were not open source.
- [VueTailwind](https://www.vue-tailwind.com/) would have been the perfect match, but it did not support Vue 3.
- [Naive UI](https://www.naiveui.com/) did not use TailwindCSS for styling,
  which is annoying for responsive things.

Several of them have caught up since then, but the choice has not been made
again. Components are mostly taken from [Flowbite](https://flowbite.com/),
copied and pasted, or taken from Headless UI and styled like Flowbite.

The use of TailwindCSS is also a strong choice. Their
[documentation](https://tailwindcss.com/docs/utility-first) explains
this choice. It makes sense, but this is sometimes a burden. Many
components are scattered around the web, and when there is no need for
JS, it is just a matter of copying, pasting, and customizing.

### Filter language

The filter box accepts an SQL-like language, described in the [console
reference](52-console.md#filter-language). The grammar is in
`console/filter/parser.peg` and the parser is generated with
[pigeon](https://github.com/mna/pigeon), which builds a parser from a parsing
expression grammar. Such grammars are easier to write than context-free ones and
they give better error messages.

Each rule carries a Go action returning a piece of the syntax tree for the
ClickHouse `WHERE` clause, and the values are checked while parsing. For
example, `ExporterAddress = 203.0.113.15` becomes `ExporterAddress =
IPv6StringToNum('203.0.113.15')`, and a malformed address is rejected at this
point.

The editor is [CodeMirror](https://codemirror.net/6/), with a second and simpler
grammar for [Lezer](https://lezer.codemirror.net/) in
`console/frontend/src/codemirror/lang-filter/`. It only needs to recognize
columns, operators and values, but it parses as the user types and accepts a
half-written expression. Linting is delegated to the server through
`/api/v0/console/filter/validate`. Completion is shared between the two sides:
the Lezer tree tells what is being typed, then
`/api/v0/console/filter/complete` returns the candidates, which may come from
ClickHouse itself for values like AS numbers.

The reasons behind this design are detailed in [a blog
post](https://vincent.bernat.ch/en/blog/2023-sql-like-language-filter).

## Other components

The core component is the main processing component in the outlet service. 
It takes metadata, routing, and other components as dependencies and 
orchestrates the flow enrichment and classification process.

The HTTP component (`common/httpserver`) exposes a web server. Its main role is
to manage the lifecycle of the HTTP server and to provide a method to add
handlers. Every service embeds one. The console uses it to serve the
single-page application, the documentation and the REST API. Other components
may expose various endpoints. They are documented in the [command line
section](51-usage.md).

The daemon component handles the lifecycle of the whole application.
It watches for the various goroutines (through tombs, see below)
that are spawned by the other components and waits for signals to terminate. If
*Akvorado* had a systemd integration, it would also take place here.

## Other interesting dependencies

- [gopkg.in/tomb.v2](https://gopkg.in/tomb.v2) handles clean goroutine
  tracking and termination. Like contexts, it allows signaling the
  termination of a bunch of goroutines. Unlike contexts, it also
  enables us to catch errors in goroutines and react to them (most of
  the time by dying).
- [github.com/benbjohnson/clock](https://github.com/benbjohnson/clock) is
  used instead of the `time` module when we want to be able to mock
  the clock. This is used, for example, in the console and in the BMP
  provider. Newer tests rely on `testing/synctest` instead, which controls time
  without changing the code under test.
- [github.com/cenkalti/backoff/v5](https://github.com/cenkalti/backoff)
  provides an exponential backoff algorithm for retries.
- [github.com/eapache/go-resiliency](https://github.com/eapache/go-resiliency)
  implements several resiliency patterns, including the breaker
  pattern.
- [github.com/go-playground/validator](https://github.com/go-playground/validator)
  implements struct validation with tags. We use it to have better
  validation on configuration structures.

[go-archaius]: https://github.com/go-chassis/go-archaius
[Harvester]: https://github.com/beatlabs/harvester
[Flowhouse]: https://github.com/bio-routing/flowhouse

## Dependency versions

As a rule of thumb:

- Go, JavaScript, Docker and GitHub action dependencies are updated by Renovate,
  configured in `.github/renovate.json`. It runs once a week and only proposes
  releases that are at least 5 days old. Most updates wait for an approval on
  the dependency dashboard. A few groups are merged automatically:
  `golang.org/x`, the frontend development tools, TypeScript, and the digests of
  the GitHub actions.
- External Docker images, like ClickHouse and Kafka, are updated to LTS versions
  when available (like for ClickHouse) or to track a supported version (like
  Kafka). There is `make docker-upgrade-versions` that updates `docker/versions.yml`.
- Go is updated when it makes sense to use some of the new features
- NodeJS is updated to the version present in Debian unstable

A good site to check if a version is still supported is
[endoflife.date](https://endoflife.date).
