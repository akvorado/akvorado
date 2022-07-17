# Troubleshooting

## Inlet service

The inlet service outputs some logs and exposes some counters to help
troubleshoot most issues. The first step to check if everything works
as expected is to request a flow:

```console
$ curl -s http://akvorado/api/v0/inlet/flows\?limit=1
{
 "TimeReceived": 1648305235,
 "SequenceNum": 425385846,
 "SamplingRate": 30000,
[...]
```

This returns the next flow. The same information is exported to Kafka.
If this does not work, be sure to check the logs and the metrics. The
later can be queried with `curl`:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet'
```

### No packets received

When running inside Docker, *Akvorado* may be unable to receive
packets because the kernel redirects these packets to Docker internal
proxy. This can be fixed by flushing the conntrack table:

```console
$ conntrack -D -p udp --orig-port-dst 2055
```

To check that you are receiving packets, check the metrics:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_flow_input_udp_packets'
```

### Wrong IP address reported for exporters

When running inside Docker, *Akvorado* may report the wrong IP address
for exporters, making it unable to query them with SNMP. This is
because Docker sets up a proxy to intercept these packets and forward
them. This can also be fixed by flushing the conntrack table:

```console
$ conntrack -D -p udp --orig-port-dst 2055
```

To check that you are now receiving packets from the right IP address, use:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_flow_input_udp_packets'
```

### No packets exported

*Akvorado* only exports packets with complete information. You can
check the metrics to find the cause:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet' | grep _errors
```

Here is a list of generic errors you may find:

- `SNMP cache miss` means the information about an interface is not
  found in the SNMP cache. This is expected when Akvorado starts but
  it should not increase. If this is the case, it may be because the
  index provided in the flow is not available through SNMP.
- `sampling rate missing` means the sampling rate information is not
  present. This is also expected when Akvorado starts but it should
  not increase. With NetFlow, the sampling rate is sent in an options
  data packet. Be sure to configure your exporter to send them (look
  for `sampler-table` in the documentation).
- `input interface missing` means the flow does not contain the input
  interface index. This is something to fix on the exporter.

When using NetFlow, you also have the `template not found` error. This
is expected on start, but then it should not increase anymore.

If *Akvorado* is unable to poll a exporter, no flows about it will be
exported. In this case, the logs contain information such as:

- `exporter:172.19.162.244 poller breaker open`
- `exporter:172.19.162.244 unable to GET`

The `akvorado_inlet_snmp_poller_failure_requests` metric would also increase
for the affected exporter.

Check that flow are correctly accepted with:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_core_flows_forwarded'
$ curl -s http://akvorado/api/v0/inlet/flows\?limit=1
```

You can check they are correctly forwarded to Kafka with:

```console
$ curl -s http://akvorado/api/v0/inlet/metrics | grep '^akvorado_inlet_kafka_sent_messages_total'
```

### Dropped packets under load

There are various bottlenecks leading to dropped packets. This is bad
as the reported sampling rate is incorrect and we cannot reliably
infer the number of bytes and packets.

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

Inside the inlet service, parsed packets are transmitted to one module
to another using channels. When there is a bottleneck at this level,
the `akvorado_inlet_flow_input_udp_out_drops` counter will increase.
There are several ways to fix that:

- increasing the channel between the input module and the flow module,
  with the `queue-size` setting attached to the input,
- increasing the number of workers for the `core` module,
- increasing the number of partitions used by the target Kafka topic,
- increasing the `queue-size` setting for the Kafka module (this can
  only be used to handle spikes).

#### SNMP poller

To process a flow, the inlet service needs the interface name and
description. This information is provided by the `snmp` submodule.
When all workers of the SNMP pollers are busy, new requests are
dropped. In this case, the `akvorado_inlet_snmp_poller_busy_count`
counter is increased. To mitigate this issue, the inlet service tries
to skip exporters with too many errors to avoid blocking SNMP requests
for other exporters. However, ensuring the exporters accept to answer
requests is the first fix. If not enough, you can increase the number
of workers. Workers handle SNMP requests synchronously.

## Kafka

There is no easy way to look at the content of the flows in a Kafka
topic. However, the metadata can be read using
[kcat](https://github.com/edenhill/kcat/). You can check a topic is
alive with:

```console
$ kcat -b kafka:9092 -C -t flows-v2 -L
Metadata for flows-v2 (from broker -1: kafka:9092/bootstrap):
 1 brokers:
  broker 1001 at eb6c7781b875:9092 (controller)
 1 topics:
  topic "flows-v2" with 4 partitions:
    partition 0, leader 1001, replicas: 1001, isrs: 1001
    partition 1, leader 1001, replicas: 1001, isrs: 1001
    partition 2, leader 1001, replicas: 1001, isrs: 1001
    partition 3, leader 1001, replicas: 1001, isrs: 1001
$ kcat -b kafka:9092 -C -t flows-v2 -f 'Topic %t [%p] at offset %o: key %k: %T\n' -o -1
```

Alternatively, when using `docker-compose`, there is a Kafka UI
running at `http://127.0.0.1:8080/kafka-ui/`. You can do the following
checks:
- are the brokers alive?
- is the `flows-v2` topic present and receiving messages?
- is ClickHouse registered as a consumer?

## ClickHouse

First, check that all the tables are present using the following SQL
query through `clickhouse client` (when running with `docker-compose`,
you can use `docker-compose exec clickhouse clickhouse-client`) :

```sql
SHOW tables
```

You should have a few tables, including `flows`, `flows_1m0s` (and
others), and `flows_2_raw`. If one is missing, look at the log in the
orchestrator. This is the component creating the tables.

To check if ClickHouse is late, use the following SQL query through
`clickhouse client` to get the lag in seconds.

```sql
SELECT (now()-max(TimeReceived))/60
FROM flows
```

If the lag is too big, you need to increase the number of consumers. See
[ClickHouse configuration](02-configuration.md#clickhouse) for details.

Another way to achieve the same thing is to look at the consumer group
from Kafka's point of view:

```console
$ kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --group clickhouse

GROUP           TOPIC           PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG             CONSUMER-ID                                                                        HOST            CLIENT-ID
clickhouse      flows-v2        0          5650351527      5650374314      22787           ClickHouse-ee97b7e7e5e0-default-flows_2_raw-0-77740d0a-79b7-4bef-a501-25a819c3cee4 /240.0.4.8      ClickHouse-ee97b7e7e5e0-default-flows_2_raw-0
clickhouse      flows-v2        3          3035602619      3035628290      25671           ClickHouse-ee97b7e7e5e0-default-flows_2_raw-3-1e4629b0-69a3-48dd-899a-20f4b16be0a2 /240.0.4.8      ClickHouse-ee97b7e7e5e0-default-flows_2_raw-3
clickhouse      flows-v2        2          1645914467      1645930257      15790           ClickHouse-ee97b7e7e5e0-default-flows_2_raw-2-79c9bafe-fd36-42fe-921f-a802d46db684 /240.0.4.8      ClickHouse-ee97b7e7e5e0-default-flows_2_raw-2
clickhouse      flows-v2        1          889117276       889129896       12620           ClickHouse-ee97b7e7e5e0-default-flows_2_raw-1-f0421bbe-ba13-49df-998f-83e49045be00 /240.0.4.8      ClickHouse-ee97b7e7e5e0-default-flows_2_raw-1
```
