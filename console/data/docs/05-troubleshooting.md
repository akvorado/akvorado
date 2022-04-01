# Troubleshooting

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
$ curl -s http://akvorado/api/v0/inlet/metrics
```

## No packets received

When running inside Docker, *Akvorado* may be unable to receive
packets because the kernel redirects these packets to Docker internal
proxy. This can be fixed by flushing the conntrack table:

```console
$ conntrack -D -p udp --orig-port-dst 2055
```

## No packets exported

*Akvorado* only exports packets with complete interface information.
They are polled through SNMP. If *Akvorado* is unable to poll a
exporter, no flows about it will be exported. In this case, the logs
contain information such as:

- `exporter:172.19.162.244 poller breaker open`
- `exporter:172.19.162.244 unable to GET`

The `akvorado_inlet_snmp_poller_failure_requests` metric would also increase
for the affected exporter.

## Dropped packets

There are various bottlenecks leading to dropped packets. This is bad
as the reported sampling rate is incorrect and we cannot reliably
infer the number of bytes and packets.

### Bottlenecks on the exporter

The first problem may come from the exporter dropping some of the
flows. Most of the time, there are counters to detect this situation
and it can be solved by lowering the exporter rate.

#### NCS5500 routers

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

### Kernel receive buffers

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

### Internal queues

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

### SNMP poller

To process a flow, the inlet service needs the interface name and
description. This information is provided by the `snmp` submodule.
When all workers of the SNMP pollers are busy, new requests are
dropped. In this case, the `akvorado_inlet_snmp_poller_busy_count`
counter is increased. To mitigate this issue, the inlet service tries
to skip exporters with too many errors to avoid blocking SNMP requests
for other exporters. However, ensuring the exporters accept to answer
requests is the first fix. If not enough, you can increase the number
of workers. Workers handle SNMP requests synchronously.
