# Internal design

*Akvorado* is written in Go. It uses a component architecture. The
entry point is `cmd/serve.go` and each directory is a distinct
component. This is heavily inspired by the [Component framework in
Clojure][]. A component is a piece of software with its configuration,
its state and its dependencies on other components.

[Component framework in Clojure]: https://github.com/stuartsierra/component

![General design](../assets/images/design.svg)

Each component features the following piece of code:

- A `Component` structure containing its state.
- A `Configuration` structure containing the configuration of the
  component. It maps to a section of [Akvorado configuration
  file](02-configuration.md).
- A `DefaultConfiguration` variable with the default values for the
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

The CLI is handled by [Cobra](https://github.com/spf13/cobra). The
configuration file is handled by
[mapstructure](https://github.com/mitchellh/mapstructure).

## Flow decoding

Decoding is handled by
[GoFlow2](https://github.com/NetSampler/GoFlow2). The network code to
receive flows is heavily inspired but was not reused. While logging is
often abstracted, this is not the case for metrics. Moreover, the
design to scale is a bit different as *Akvorado* will create a socket
for each worker instead of distributing incoming flows using message
passing.

Only Netflow v9 and IPFIX are currently handled. However, as *GoFlow2*
also supports sFlow, support can be added later.

The design of this component is modular as it is possible to "plug"
new decoders and new inputs easily. It is expected that most buffering
to be done at this level by input modules that need them. However,
some buffering also happens in the Kafka module. When the input is the
network, this does not really matter as we cannot really block without
losing messages. But with file-backed modules, it may be more reliable
to not have buffers elsewhere as they can be lost during shutdown.

## GeoIP

The component is mostly boring, with the exception of having a
goroutine watching for the modification of the databases to update
them.

## Kafka

The Kafka component relies on
[Sarama](https://github.com/Shopify/sarama). It is tested using the
mock interface provided by this package. *Sarama* uses `go-metrics` to
store metrics. We convert them to Prometheus to keep them.

If a real broker is available under the DNS name `kafka` or at
`localhost` on port 9092, it will be used for a quick functional test.

## ClickHouse

The ClickHouse manages migrations for the ClickHouse database. It
relies on [migrate](https://github.com/golang-migrate/migrate) with a
simplified ClickHouse driver (the original one does not work with
ClickHouse v2) and a custom source driver allowing to use templates.

I have later discovered the [ClickHouse
client](https://github.com/uptrace/go-clickhouse) from Uptrace which
also features
[migrations](https://clickhouse.uptrace.dev/guide/migrations.html) but
allows us to use Go code in additional to SQL text files. It may help
being smarter with migrations in the future.

Functional tests are run when a ClickHouse server is available under
the name `clickhouse` or on `localhost`.

## SNMP

SNMP polling is done with [GoSNMP](https://github.com/gosnmp/gosnmp).
The cache layer is tailored specifically for our needs. Information
contained in it expires if not accessed and is refreshed periodically
otherwise. Some coaelescing of the requests are done when they are
piling up. This adds some code complexity, maybe it was not worth it.
If a exporter fails to answer too frequently, it will be blacklisted
for a minute just to ensure it does not eat up all the workers'
capacity.

Testing is done by another implementation of an [SNMP
agent](https://github.com/salyercat/GoSNMPServer).

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
   implements several resiliency pattersn, including the breaker
   pattern.

## Future plans

In the future, we may:

- Add more information to the landing page, including some basic statistics.
- Automatically build dashboards for Grafana.[^grafana]
- Builds dashboards with [D3.js][].[^d3js]
- Buffer message to disks instead of blocking (when sending to Kafka)
  or dropping (when querying the SNMP poller). We could probable just
  have a system service running tcpdump dumping packets to a directory
  and use that as input. This would be allow *Akvorado* to block from
  end-to-end instead of trying to be realtime.
- Collect routes by integrating GoBGP. This is low priority if we
  consider information from Maxmind good enough for our use.

[^grafana]: The templating system in Grafana is quite limited.
    Notably, it is difficult to build different query depending on the
    input fields. Grafana supports scripted dashboard, but it does not
    seem to be possible to have a function build the query string.
[^d3js]: There is a [gallery][] containing many interesting examples,
    including [stacked area charts][], [small multiple charts][] and
    [Sankey diagrams][].

[D3.js]: https://d3js.org/
[gallery]: https://www.d3-graph-gallery.com/
[stacked area charts]: https://www.d3-graph-gallery.com/stackedarea.html
[small multiple charts]: https://www.d3-graph-gallery.com/graph/area_smallmultiple.html
[Sankey diagrams]: https://www.d3-graph-gallery.com/graph/sankey_basic.html
