# Design

<figure markdown>
  ![General design](assets/images/design.svg){ width="500" }
  <figcaption>General design for Akvorado</figcaption>
</figure>

## Big picture

The general design of *Akvorado* is the following:

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

- A ClickHouse database subscribe to the Kafka topic to receive and
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
[expression language]: https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md
[D3.js]: https://d3js.org/
[gallery]: https://www.d3-graph-gallery.com/
[stacked area charts]: https://www.d3-graph-gallery.com/stackedarea.html
[small multiple charts]: https://www.d3-graph-gallery.com/graph/area_smallmultiple.html
[Sankey diagrams]: https://www.d3-graph-gallery.com/graph/sankey_basic.html
