# Design

## Big picture

The general design of *Akvorado* is the following:

<figure markdown>
  ![General design](assets/images/design.svg){ width="500" }
  <figcaption>General design for Akvorado</figcaption>
</figure>

- The samplers send flow to Akvorado. They don't need to be declared.
- The received flows are decoded then sent to the core component.
- For each flow, the core component query the GeoIP component and the
  SNMP poller to get additional information, including country, AS
  number, interface name, description and speed.
- The GeoIP component provides country and AS information for IP
  addresses using Maxmind databases.
- The SNMP poller queries the samplers for host names, interface
  names, interface descriptions and interface speeds. This information
  is cached and updated from time to time.
- Once the core component has a complete flow, it pushes it to the
  Kafka component.
- The Kafka component turns the flow into a binary representation
  using *protocol buffers* and send the result into a Kafka topic.

The remaining steps are outside of *Akvorado* control:

- A Clickhouse database subscribe to the Kafka topic to receive and
  process the flows.
- A graphing tool like Grafana queries this database to build various
  dashboards.

## Flow representation

The flow representation is encoded in a versioned `flow-*.proto` file.
Any information that could change with time is embedded in the flow.
This includes for example interface names and speeds, as well. This
ensures that older data are not processed using incorrect mappings.

Each time the schema changes, we issue a new `flow-*.proto` file,
update the schema version and a new Kafka topic will be used. This
ensures we do not mix different schemas in a single topic.

## Programming design

*Akvorado* is written in Go. It uses a component architecture. The
entry point is `cmd/serve.go` and each directory is a distinct
component. This is heavily inspired by the [Component framework in
Clojure][]. A component is a piece of software with its configuration,
its state and its dependencies on other components.

[Component framework in Clojure]: https://github.com/stuartsierra/component

Each component features the following piece of code:

- A `Component` structure containing its state.
- A `Configuration` structure containing the configuration of the
  component. It maps to a section of [Akvorado configuration
  file][configuration.md].
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

### Reporter

The reporter is a special component handling logs and metrics for all
the other components. In the future, this could also be the place to
handle crash reports.

For logs, it is mostly a façade to
[github.com/rs/zerolog](https://github.com/rs/zerolog) with some additional
code to append the module name to the logs.

For metrics, it is a façade to the [Prometheus instrumentation
library][]. It provides a registry which automatically append metric
names with the module name.

The general idea is to give a good visibility to an operator.
Everything that moves should get a counter, errors should either be
fatal, or rate-limited and accounted into a metric.

[Prometheus instrumentation library]: https://github.com/prometheus/client_golang/

### CLI

The CLI is handled by [Cobra](https://github.com/spf13/cobra). The
configuration file is handled by
[mapstructure](https://github.com/mitchellh/mapstructure).

### Flow decoding

Decoding is handled by
[GoFlow2](https://github.com/NetSampler/GoFlow2). The network code to
receive flows is heavily inspired but was not reused. While logging is
often abstracted, this is not the case for metrics. Moreover, the
design to scale is a bit different as *Akvorado* will create a socket
for each worker instead of distributing incoming flows using message
passing.

Only Netflow v9 is currently handled. However, as *GoFlow2* also
supports sFlow and IPFIX, support for them can be added later.

### GeoIP

The component is mostly boring, with the exception of having a
goroutine watching for the modification of the databases to update
them.

### Kafka

The Kafka component relies on
[Sarama](https://github.com/Shopify/sarama). It is tested using the
mock interface provided by this package. No tests are running against
a real Kafka broker. *Sarama* uses `go-metrics` to store metrics. We
convert them to Prometheus to keep them.

### SNMP

SNMP polling is done with [GoSNMP](https://github.com/gosnmp/gosnmp).
The cache layer is tailored specifically for our needs. Information
contained in it will be refreshed before expiring. However, currently,
no check is done if we still need the information. So, the information
can only expire when it does not exist on the network, not when we
don't need it.

Testing is done by another implementation of an [SNMP
agent](https://github.com/salyercat/GoSNMPServer).

### Other components

The core component is the main one. It takes the other as dependencies
but there is nothing exciting about it.

The HTTP component exposes a web server. Its main role is to manage
the lifecycle of the HTTP server and to provide a method to add
handlers. The web component provides the web interface of *Akvorado*.
Currently, this is only the documentation. Other components may expose
some various endpoints. They are documented in the [usage
section](usage.md).

The daemon component handles the lifecycle of the whole application.
It watches for the various goroutines (through tombs, see below)
spawned by the other components and wait for signals to terminate. If
*Akvorado* had a systemd integration, it would take place here too.

### Other interesting dependencies

 - [gopkg.in/tomb.v2](https://gopkg.in/tomb.v2) handles clean goroutine
   tracking and termination. Like contexts, it allows to signal
   termination of a bunch of goroutines. Unlike contexts, it also
   enables us to catch errors in goroutines and react to them (most of
   the time by dying).
 - [github.com/benbjohnson/clock](https://github.com/benbjohnson/clock) is
   used in place of the `time` module when we want to be able to mock
   the clock. This is used for example to test the cache of the SNMP
   poller.
 - [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
   is used  when some rate limiting is needed, notably for non-fatal
   errors.
 - [github.com/cenkalti/backoff/v4](https://github.com/cenkalti/backoff)
   provides an exponential backoff algorithm for retries.

## Future plans

In the future, we may:

- Add more information to the landing page, including some basic statistics.
- Automatically build dashboards for Grafana.[^grafana]
- Builds dashboards with [D3.js][].[^d3js]
- Manage the other components (Kafka topic creation, Clickhouse
  configuration) to make deployments easier.
- Collect routes by integrating GoBGP. This is low priority if we
  consider information from Maxmind good enough for our use.

[^grafana]: The templating system in Grafana is quite limited.
    Notably, it is difficult to build different query depending on the
    input fields. Grafana supports scripted dashboard, but it does not
    seem to be possible to have a function build the query string.
[^d3js]: There is a [gallery][] containing many interesting examples,
    including [stacked area charts][], [small multiple charts][] and
    [Sankey diagrams][].
[expression language]: https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md
[D3.js]: https://d3js.org/
[gallery]: https://www.d3-graph-gallery.com/
[stacked area charts]: https://www.d3-graph-gallery.com/stackedarea.html
[small multiple charts]: https://www.d3-graph-gallery.com/graph/area_smallmultiple.html
[Sankey diagrams]: https://www.d3-graph-gallery.com/graph/sankey_basic.html
