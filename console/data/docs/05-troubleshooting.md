# Troubleshooting

## Inlet service

The inlet service receives Netflow/IPFIX/sFlow packets and forwards them to
Kafka. It outputs some logs and exposes some counters to help troubleshoot
packet reception issues. The metrics can be queried with `curl`:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet'
```

Be sure to replace `http://akvorado` with the URL to your *Akvorado*
setup. If you are running `docker compose` locally, this is
`http://127.0.0.1:8080`.

### No packets received

When running inside Docker, *Akvorado* may be unable to receive
packets because the kernel redirects these packets to Docker internal
proxy. This can be fixed by flushing the conntrack table:

```console
$ conntrack -D -p udp --orig-port-dst 2055
```

The shipped `docker-compose.yml` file contains an additional service
to do that automatically.

To check that you are receiving packets, check the metrics:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_flow_input_udp_packets'
```

The inlet service only receives and forwards packets - it doesn't perform
SNMP queries or flow enrichment. These are handled by the outlet service.

### No packets forwarded to Kafka

The inlet service receives packets and forwards them to Kafka. Check that 
flows are correctly forwarded to Kafka with:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_kafka_sent_messages_total'
```

## Outlet service

The outlet service consumes flows from Kafka, parses them, enriches them 
with metadata and routing information, and exports them to ClickHouse.
To check if the outlet is working correctly, request a processed flow:

```console
$ curl -s http://akvorado/api/v0/outlet/flows\?limit=1
{
 "TimeReceived": 1648305235,
 "SamplingRate": 30000,
[...]
```

This returns the next processed flow with all enrichment applied. If this
doesn't return flows, check the outlet metrics:

```console
$ curl -s http://akvorado/api/v0/outlet/metrics | grep '^akvorado_outlet'
```

### No flows received

First, check if there are flows received from Kafka.

```console
$ curl -s http://akvorado/api/v0/outlet/metrics | grep '^akvorado_outlet_kafka' | grep received
```

Another way to achieve the same thing is to look at the consumer group
from Kafka's point of view:

```console
$ kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --group akvorado-outlet

GROUP           TOPIC           PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG             CONSUMER-ID                                                      HOST            CLIENT-ID
akvorado-outlet flows-v5        0          5650351527      5650374314      22787           akvorado-outlet-flows-v5-0-77740d0a-79b7-4bef-a501-25a819c3cee4  /240.0.4.8      akvorado-oulet-flows-v5
akvorado-outlet flows-v5        3          3035602619      3035628290      25671           akvorado-outlet-flows-v5-3-1e4629b0-69a3-48dd-899a-20f4b16be0a2  /240.0.4.8      akvorado-oulet-flows-v5
akvorado-outlet flows-v5        2          1645914467      1645930257      15790           akvorado-outlet-flows-v5-2-79c9bafe-fd36-42fe-921f-a802d46db684  /240.0.4.8      akvorado-oulet-flows-v5
akvorado-outlet flows-v5        1          889117276       889129896       12620           akvorado-outlet-flows-v5-1-f0421bbe-ba13-49df-998f-83e49045be00  /240.0.4.8      akvorado-oulet-flows-v5
```

### No flows processed

The outlet service only exports flows with complete information. You can
check the metrics to find the cause:

```console
$ curl -s http://akvorado/api/v0/outlet/metrics | grep '^akvorado_outlet' | grep _error
```

Here is a list of generic errors you may find:

- `SNMP cache miss` means the information about an interface is not
  found in the SNMP cache. This is expected when Akvorado starts but
  it should not increase. If this is the case, it is likely because
  the exporter is not configured to accept SNMP requests or the
  community configured for SNMP is incorrect.
- `sampling rate missing` means the sampling rate information is not present.
  This is also expected when Akvorado starts but it should not increase. With
  NetFlow, the sampling rate is sent in an options data packet. Be sure to
  configure your exporter to send them (look for `sampler-table` in the
  documentation). Alternatively, you can configure
  `outlet`→`core`→`default-sampling-rate` to workaround this issue.
- `input and output interfaces missing` means the flow does not contain the
  input and output interface indexes. This is something to fix on the exporter.

If the outlet service is unable to poll an exporter, no flows about it will be
exported. In this case, the logs contain information such as:

- `exporter:172.19.162.244 poller breaker open`
- `exporter:172.19.162.244 unable to GET`

The `akvorado_outlet_metadata_provider_snmp_error_requests_total` metric would also
increase for the affected exporter. If your routers are in
`172.16.0.0/12` and you are using Docker, Docker subnets may overlap
with your routers'. To avoid this, you can put that in
`/etc/docker/daemon.json` and restart Docker:

```json
{
 "default-address-pools": [{"base":"240.0.0.0/16","size":24}],
 "userland-proxy": false
}
```

If the exporter address is incorrect, the above configuration will also help.

Check that flows are correctly processed with:

```console
$ curl -s http://akvorado/api/v0/outlet/metrics | grep '^akvorado_outlet_core_forwarded_flows_total'
$ curl -s http://akvorado/api/v0/outlet/flows\?limit=1
```

### Reported traffic levels are incorrect

Use `curl -s http://akvorado/api/v0/outlet/flows\?limit=1 | grep
SamplingRate` to check if the reported sampling rate is correct. If
not, you can override it with `outlet`→`core`→`override-sampling-rate`.

Another cause possible cause is when your router is configured to send
flows for both an interface and its parent. For example, if you have
an LACP-enabled interface, you should collect flows only for the
aggregated interface, not for the individual sub interfaces.

### No traffic visible on the web interface

You may see the last flow widget correctly populated, but nothing else. The
various widgets on the home page are relying on interface classification to
retrieve information. Notably, they expect `InIfBoundary` or `OutIfBoundary` to
be set to external. You can check that classification is done correctly by
removing any filter rule on the interface and by grouping on `InIfBoundary` (for
example). If not, be sure that your rules are correct and that descriptions
match what you expect. For example, on Juniper, if you enable JFlow on a
sub-interface, be sure that the description is present on this sub-interface.

### 4-byte ASN 23456 in flow data

If you are seeing flows with source or destination AS of 23456, your exporter 
needs to be configured with 4-byte ASN support. For example, on Cisco IOS-XE:

```cisco
flow record Akvorado
    collect routing source as 4-octet
    collect routing destination as 4-octet
!
```

### Dropped packets under load

There are various bottlenecks leading to dropped packets. This is bad
as the reported sampling rate is incorrect and we cannot reliably
infer the number of bytes and packets.

Most packet drops occur at the inlet service (packet reception) while
processing bottlenecks occur at the outlet service (flow enrichment).

#### Bottlenecks on the exporter

The first problem may come from the exporter dropping some of the
flows. Most of the time, there are counters to detect this situation
and it can be solved by lowering the exporter rate.

##### NCS5500 routers

[Netflow, Sampling-Interval and the Mythical Internet Packet Size][1]
contains many information about the limit of this platform. The first
bottleneck is a 133 Mbps shaper between an NPU and the LC CPU for the
sampled packets (144 bytes each). For example, on a NC55-36X100G line
card, there are 6 NPU, each one managing 6 ports. If we consider an
average packet size of 1000, the maximum sampling rate when all ports
are full is 1:700 (formula is `Total-BW / ( Avg-Pkt-Size x 133Mbps ) x
( 144 x 8 )`).

[1]: https://xrdocs.io/ncs5500/tutorials/2018-02-19-netflow-sampling-interval-and-the-mythical-internet-packet-size/

It is possible to check if there are drops with `sh controllers npu
stats voq base 24 instance 0 location 0/0/CPU0` and looking at the
`COS2` line.

The second bottleneck is the size of the flow cache. If too small, it
may overflow. For example:

```console
# show flow monitor monitor1 cache internal location 0/1/CPU0 | i Cache
Cache summary for Flow Monitor :
Cache size:                         100000
Cache Hits:                            202938943
Cache Misses:                         1789836407
Cache Overflows:                         2166590
Cache above hi water:                       1704
```

When this happens, either the `cache timeout rate-limit` should be
increased or the `cache entries` directive should be increased. The
later value can be increased to 1 million par monitor-map.

#### Kernel receive buffers

The second source of drops are the kernel receive buffers. Each
listening queue has a fixed amount of receive buffers (212992 bytes by
default) to keep packets before handling them to the application. When
this buffer is full, packets are dropped.

*Akvorado* reports the number of drops for each listening socket with
the `akvorado_inlet_flow_input_udp_in_drops` counter. This should be
compared to `akvorado_inlet_flow_input_udp_packets`. Another way to get the same
information is by using `ss -lunepm` and look at the drop counter:

```console
$ nsenter -t $(pidof akvorado) -n ss -lunepm
State            Recv-Q           Send-Q                       Local Address:Port                        Peer Address:Port           Process
UNCONN           0                0                                        *:2055                                   *:*               users:(("akvorado",pid=2710961,fd=16)) ino:67643151 sk:89c v6only:0 <->
         skmem:(r0,rb212992,t0,tb212992,f4096,w0,o0,bl0,d486525)
```

In the example above, there were 486525 drops. This can be solved
either by increasing the number of workers for the UDP input or by
increasing the value of `net.core.rmem_max` sysctl and increasing the
`receive-buffer` setting attached to the input.

#### Internal queues

Inside the inlet service, received packets are transmitted from the input 
module to the Kafka module using channels. When there is a bottleneck at 
this level, the `akvorado_inlet_flow_input_udp_out_drops` counter will increase.
There are several ways to fix that:

- increasing the channel between the input module and the Kafka module,
  with the `queue-size` setting attached to the input,
- increasing the number of partitions used by the target Kafka topic,
- increasing the `queue-size` setting for the Kafka module (this can
  only be used to handle spikes).

Inside the outlet service, flows are transmitted between the Kafka consumer,
core processing, and ClickHouse modules. When there are bottlenecks,
the `akvorado_outlet_*_drops` counters will increase. These can be fixed by:

- increasing the number of workers for the `core` module,
- increasing the number of Kafka consumers,
- tuning ClickHouse insertion parameters.

#### SNMP poller

To process a flow, the outlet service needs the interface name and
description. This information is provided by the `metadata` submodule.
When all workers of the SNMP pollers are busy, new requests are
dropped. In this case, the `akvorado_outlet_metadata_provider_busy_count`
counter is increased. To mitigate this issue, the outlet service tries
to skip exporters with too many errors to avoid blocking SNMP requests
for other exporters. However, ensuring the exporters accept to answer
requests is the first fix. If not enough, you can increase the number
of workers. Workers handle SNMP requests synchronously.

### Profiling

On a large scale installation, you may want to check if *Akvorado* is using too
much CPU or memory. This can be achieved with `pprof`, the [Go
profiler](https://go.dev/blog/pprof). You need a working [installation of
Go](https://go.dev/doc/install) on your workstation.

When running on Docker, use `docker inspect` to get the IP address of the service 
you want to profile (inlet or outlet):

```console
$ docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' akvorado_akvorado-inlet_1
240.0.4.8
$ docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' akvorado_akvorado-outlet_1
240.0.4.9
```

Then, use one of the two following commands:

```console
$ go tool pprof http://240.0.4.8:8080/debug/pprof/profile
$ go tool pprof http://240.0.4.8:8080/debug/pprof/heap
```

If your Docker host is remote, you also need to use SSH forwarding to expose the
HTTP port to your workstation:

```console
$ ssh -L 6060:240.0.4.8:8080 dockerhost.example.com
```

Then, use one of the two following commands:

```console
$ go tool pprof http://127.0.0.1:6060/debug/pprof/profile
$ go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

The first one provides a CPU profile. The second one a memory profile. On the
command-line, you can type `web` to visualize the result in the browser or `svg`
to get a SVG file you can attach to a bug report if needed.

## Kafka

There is no easy way to look at the content of the flows in a Kafka
topic. However, the metadata can be read using
[kcat](https://github.com/edenhill/kcat/). You can check a topic is
alive with:

```console
$ kcat -b kafka:9092 -C -t flows-v5 -L
Metadata for flows-v5 (from broker -1: kafka:9092/bootstrap):
 1 brokers:
  broker 1001 at eb6c7781b875:9092 (controller)
 1 topics:
  topic "flows-v5" with 4 partitions:
    partition 0, leader 1001, replicas: 1001, isrs: 1001
    partition 1, leader 1001, replicas: 1001, isrs: 1001
    partition 2, leader 1001, replicas: 1001, isrs: 1001
    partition 3, leader 1001, replicas: 1001, isrs: 1001
$ kcat -b kafka:9092 -C -t flows-v5 -f 'Topic %t [%p] at offset %o: key %k: %T\n' -o -1
```

Alternatively, when using `docker compose`, there is a Kafka UI
running at `http://127.0.0.1:8080/kafka-ui/`. You can do the following
checks:

- are the brokers alive?
- is the `flows-v5` topic present and receiving messages?
- is Akvorado registered as a consumer?

## ClickHouse

First, check that all the tables are present using the following SQL
query through `clickhouse client` (when running with `docker compose`,
you can use `docker compose exec clickhouse clickhouse-client`) :

```sql
SHOW tables
```

You should have a few tables, including `flows`, `flows_1m0s`, and others. If
one is missing, look at the log in the orchestrator This is the component
creating the tables.

To check if ClickHouse is late, use the following SQL query through
`clickhouse client` to get the lag in seconds.

```sql
SELECT (now()-max(TimeReceived))/60
FROM flows
```

If you still have an issue, be sure to check the errors reported by
ClickHouse:

```sql
SELECT last_error_time, last_error_message
FROM system.errors
ORDER BY last_error_time LIMIT 10
FORMAT Vertical
```
