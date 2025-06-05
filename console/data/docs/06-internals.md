# Internal design

*Akvorado* is written in Go. Each service has its code in a distinct
directory (`inlet/`, `orchestrator/` and `console/`). The `common/`
directory contains components common to several services. The `cmd/`
directory contains the main entry points.

Each service is splitted into several components. This is heavily
inspired by the [Component framework in Clojure][]. A component is a
piece of software with its configuration, its state and its
dependencies on other components.

[Component framework in Clojure]: https://github.com/stuartsierra/component

Each component features the following piece of code:

- A `Component` structure containing its state.
- A `Configuration` structure containing the configuration of the
  component. It maps to a section of [Akvorado configuration
  file](02-configuration.md).
- A `DefaultConfiguration` function with the default values for the
  configuration.
- A `New()` function instantiating the component. This method takes
  the configuration and the dependencies. It is inert.
- Optionally, a `Start()` method to start the routines associated to
  the component.
- Optionally, a `Stop()` method to stop the component.

Each component is tested independently. If a component is complex, a
`NewMock()` function can create a component with a compatible
interface to be used in place of the real component. In this case, it
takes a `testing.T` struct as first argument and starts the component
immediately. It could return the real component or a mocked version.
For example, the Kafka component returns a component using a mocked
Kafka producer.

Dependencies are handled manually, unlike more complex component-based
solutions like [Uber Fx][].

[Uber Fx]: https://github.com/uber-go/fx

## Reporter

The reporter is a special component handling logs and metrics for all
the other components. In the future, this could also be the place to
handle crash reports.

For logs, it is mostly a façade to
[github.com/rs/zerolog](https://github.com/rs/zerolog) with some additional
code to append the module name to the logs.

For metrics, it is a façade to the [Prometheus instrumentation
library][]. It provides a registry which automatically append metric
names with the module name.

It also exposes a simple way to report healthchecks from various
components. While it could be used to kill the application
proactively, currently, it is only exposed through HTTP. Not all
components have healthchecks. For example, for the `flow` component,
it is difficult to read from UDP while watching for a check. For the
`http` component, the healthcheck would be too trivial (not in the
routine handling the heavy work). For `kafka`, the hard work is hidden
by the underlying library and we wouldn't want to be declared
unhealthy because of a transient problem by checking broker states
manually. The `daemon` component tracks the important goroutines, so it
is not vital.

The general idea is to give a good visibility to an operator.
Everything that moves should get a counter, errors should either be
fatal, or rate-limited and accounted into a metric.

[Prometheus instrumentation library]: https://github.com/prometheus/client_golang/

## CLI

The CLI (not a component) is handled by
[Cobra](https://github.com/spf13/cobra). The configuration file is
handled by [mapstructure](https://github.com/mitchellh/mapstructure).
Handling backward compatibility is done by registering hooks to
transform the configuration.

## Flow decoding

Decoding is handled by
[GoFlow2](https://github.com/NetSampler/GoFlow2). The network code to
receive flows is heavily inspired by it but not reused. While logging is
often abstracted, this is not the case for metrics. Moreover, the
design to scale is a bit different as *Akvorado* will create a socket
for each worker instead of distributing incoming flows using a channel.

Netflow v5, Netflow v9, IPFIX, and sFlow are currently supported.

The design of this component is modular. It is possible to "plug"
new decoders and new inputs easily. It is expected that most buffering
is implemented at this level by input modules that require them.
Additionnal buffering happens in the Kafka module. When the input is the
network, this does not really matter as we cannot really block without
losing messages. However, with file-backed modules, it may be more reliable
to reduce buffers as data can be lost during shutdown.

## Kafka

The Kafka component relies on
[Sarama](https://github.com/IBM/sarama). It is tested using the
mock interface provided by this package. *Sarama* uses `go-metrics` to
store metrics. We convert them to Prometheus to keep them. The logger
is global and there is a hack to be plug it into the reporter design
we have.

If a real broker is available under the DNS name `kafka` or at
`localhost` on port 9092, it will be used for a quick functional test.

## ClickHouse

For this OLAP database, migrations are done with a simple loop
checking if a step is needed using a custom query and executing it with Go code.
Database migration systems exist in Go, notably
[migrate](https://github.com/golang-migrate/migrate), but as the
table schemas depend on user configuration, it is preferred
to use code to check if the existing tables are up-to-date
and to update them. For example, we may want to check if the Kafka
settings of a table or the source URL of a dictionary are current.

Functional tests are run when a ClickHouse server is available under
the name `clickhouse` or on `localhost`.

## SNMP

SNMP polling is accomplished with [GoSNMP](https://github.com/gosnmp/gosnmp).
The cache layer is tailored specifically for our needs. Cached information
can expire if not accessed or refreshed periodically.
Some coaelescing of the requests are done when they are queued.
This adds some code complexity, maybe it was not worth it.
If a exporter fails to answer too frequently, a backoff will be triggered
for a minute to ensure it does not eat up all the workers' resources.

Testing is done by another implementation of an [SNMP
agent](https://github.com/slayercat/GoSNMPServer).

## BMP

The BMP server uses [GoBGP](http://github.com/osrg/gobgp)'s
implementation. GoBGP does not have a BMP collector, but it's just a
simple TCP connection receiving BMP messages and we use GoBGP to parse
them. The data we need is stored in a Patricia tree.

[github.com/kentik/patricia](https://github.com/kentik/patricia)
implements a fast Patricia tree for IP lookup in a tree of subnets. It
leverages Go generics to make the code safe. It is used both for
configuring subnet-dependent settings (eg SNMP communities) and for
storing data received using BMP.

To save memory, *Akvorado* "interns" next-hops, origin AS, AS paths
and communities. Each unique combination is associated to a
reference-counter 32-bit integer, which is used in the RIB in place of
the original information.

## Schema

*Akvorado* schema is a bit dynamic. One can add or remove columns of data.
However, everything needs to be predefined in the code. To add a new column, one
needs to follow these steps:

1. Add its symbol to `common/schema/definition.go`.
2. Add it to the `flow()` function in `common/schema/definition.go`. Be sure to
   specify the right/smaller ClickHouse type. If the columns is prefixed with
   `Src` or `InIf`, don't add the opposite direction, this is done
   automatically. Use `ClickHouseMainOnly` if the column is expected to take a
   lot of space. Add the column to the end and set `Disabled` field to `true`.
   If you add several fields, create a group and use it on decoding to keep
   decoding/encoding fast for people not enabling them.
3. Make it usable in the filters by adding it to `console/filter/parser.peg`.
   Don't forget to add a test in `console/filter/parser_test.go`.
4. Modify `console/query/column.go` to alter the display of the column (it
   should be a string).
5. If it does not have a proper type in ClickHouse to be displayed as is (like a
   MAC address stored as a 64-bit integer), also modify
   `widgetFlowLastHandlerFunc()` in `console/widgets.go`.
6. Modify `inlet/flow/decoder/netflow/decode.go` and
   `inlet/flow/decoder/sflow/decode.go` to extract the data from the flows.
7. If useful, add a completion in `filterCompleteHandlerFunc()` in
   `akvorado/console/filter.go`.

## Web console

The web console is built as a REST API with a single page application
on top of it.

### REST API

The REST API is mostly built using the [Gin
framework](https://gin-gonic.com/) which removes some boilerplate
compared to using pure Go. Also, it uses the [validator
package](https://github.com/go-playground/validator) which implements
value validations based on tags. The validation options are quite
rich.

### Single page application

The SPA is built using mostly the following components:

- [TypeScript](https://www.typescriptlang.org) instead of JavaScript,
- [Vite](https://vitejs.dev/) as a builder,
- [Vue](https://vuejs.org/) as the reactive JavaScript framework,
- [TailwindCSS](https://tailwindcss.com/) for styling pages directly inside HTML,
- [Headless UI](https://headlessui.dev/) for some unstyled UI components,
- [ECharts](https://echarts.apache.org/) to plot charts.
- [CodeMirror](https://codemirror.net/6/) to edit filter expressions.

There is no full-blown component library despite the existence of many candidates:

- [Vuetify](https://vuetifyjs.com/) is only compatible with Vue 2.
- [BootstrapVue](https://bootstrap-vue.org/) is only compatible with Vue 2.
- [PrimeVue](https://www.primefaces.org/primevue/) is quite heavyweight and many stuff are not opensource.
- [VueTailwind](https://www.vue-tailwind.com/) would be the perfect match but it is not compatible with Vue 2.
- [Naive UI](https://www.naiveui.com/) may be a future option but the
  styling is not using TailwindCSS which is annoying for responsive
  stuff, but we can just stay away from the proposed layout.

So, currently, components are mostly taken from
[Flowbite](https://flowbite.com/), copy/pasted or from Headless UI and
styled like Flowbite.

Use of TailwindCSS is also a strong choice. Their
[documentation](https://tailwindcss.com/docs/utility-first) explains
this choice. It makes sense but this is sometimes a burden. Many
components are scattered around the web and when there is no need for
JS, it is just a matter of copy/pasting and customizing.

## Other components

The core component is the main one. It takes the other as dependencies
but there is nothing exciting about it.

The HTTP component exposes a web server. Its main role is to manage
the lifecycle of the HTTP server and to provide a method to add
handlers. The web component provides the web interface of *Akvorado*.
Currently, this is only the documentation. Other components may expose
some various endpoints. They are documented in the [usage
section](03-usage.md).

The daemon component handles the lifecycle of the whole application.
It watches for the various goroutines (through tombs, see below)
spawned by the other components and wait for signals to terminate. If
*Akvorado* had a systemd integration, it would take place here too.

## Other interesting dependencies

- [gopkg.in/tomb.v2](https://gopkg.in/tomb.v2) handles clean goroutine
  tracking and termination. Like contexts, it allows to signal
  termination of a bunch of goroutines. Unlike contexts, it also
  enables us to catch errors in goroutines and react to them (most of
  the time by dying).
- [github.com/benbjohnson/clock](https://github.com/benbjohnson/clock) is
  used in place of the `time` module when we want to be able to mock
  the clock. This is used for example to test the cache of the SNMP
  poller.
- [github.com/cenkalti/backoff/v4](https://github.com/cenkalti/backoff)
  provides an exponential backoff algorithm for retries.
- [github.com/eapache/go-resiliency](https://github.com/eapache/go-resiliency)
  implements several resiliency patterns, including the breaker
  pattern.
- [github.com/go-playground/validator](https://github.com/go-playground/validator)
  implements struct validation using tags. We use it to had better
  validation on configuration structures.

[go-archaius]: https://github.com/go-chassis/go-archaius
[Harvester]: https://github.com/beatlabs/harvester
[Flowhouse]: https://github.com/bio-routing/flowhouse
