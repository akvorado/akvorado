# Scale Akvorado

Various bottlenecks can cause dropped packets. This is problematic because the
reported sampling rate becomes incorrect, and you cannot reliably calculate the
number of bytes and packets. Both the exporters and the inlet need to be tuned
for this kind of problem.

The outlet can also be a bottleneck. In this case, the flows may appear on the
console with a delay.

## Exporters

The first problem may be that the exporter is dropping flows. Usually, counters
can detect this situation, and you can solve it by reducing the exporter rate.

### NCS5500 routers

[NetFlow, Sampling-Interval and the Mythical Internet Packet Size][1] contains
a lot of information about the limits of this platform. The first bottleneck is a 133
Mbps shaper between an NPU and the LC CPU for the sampled packets (144 bytes
each). For example, on a NC55-36X100G line card, there are 6 NPUs, and each one
manages 6 ports. If we consider an average packet size of 1000, the maximum
sampling rate when all ports are full is 1:700 (the formula is `Total-BW / (
Avg-Pkt-Size x 133Mbps ) x ( 144 x 8 )`).

[1]: https://xrdocs.io/ncs5500/tutorials/2018-02-19-netflow-sampling-interval-and-the-mythical-internet-packet-size/

It is possible to check if there are drops with `sh controllers npu
stats voq base 24 instance 0 location 0/0/CPU0` and looking at the
`COS2` line.

The second bottleneck is the size of the flow cache. If it is too small, it may
overflow. For example:

```console
# show flow monitor monitor1 cache internal location 0/1/CPU0 | i Cache
Cache summary for Flow Monitor :
Cache size:                         100000
Cache Hits:                            202938943
Cache Misses:                         1789836407
Cache Overflows:                         2166590
Cache above hi water:                       1704
```

When this happens, either the `cache timeout rate-limit` should be increased,
or the `cache entries` directive should be increased. The latter value can be
increased to 1 million per monitor-map.

### Other routers

Other routers are likely to have the same limitations. Note that
sFlow and IPFIX 315 do not have a flow cache and are therefore less likely to
have scaling problems.

## Inlet

When the inlet has scaling issues, the kernel\'s receive buffers may drop packets.
Each listening queue has a fixed number of receive buffers (212992 bytes by
default) to keep packets before they are handled by the application. When this
buffer is full, packets are dropped.

*Akvorado* reports the number of drops for each listening socket with the
`akvorado_inlet_flow_input_udp_in_dropped_packets_total` counter. This should be
compared to `akvorado_inlet_flow_input_udp_packets_total`. Another way to get
the same information is to use `ss -lunepm` and look at the drop counter:

```console
$ nsenter -t $(pgrep -fo "akvorado inlet") -n ss -lunepm
State            Recv-Q           Send-Q                       Local Address:Port                        Peer Address:Port           Process
UNCONN           0                0                                        *:2055                                   *:*               users:(("akvorado",pid=2710961,fd=16)) ino:67643151 sk:89c v6only:0 <->
         skmem:(r0,rb212992,t0,tb212992,f4096,w0,o0,bl0,d486525)
```

In the example above, there were 486525 drops. You can solve this in different ways:

1. Check the `akvorado_inlet_kafka_buffered_produce_records_total` gauge. If it
   is often near the value of `kafka`→`queue-size`, increase this value. This
   setting has the most impact.
1. Increase the number of workers for the UDP input.
1. Enable the eBPF load balancer on Linux (check `docker/docker-compose-local.yml`).
1. Increase the value of the `net.core.rmem_max` sysctl (on the host) and
   increase the `receive-buffer` setting for the input to the same value,
1. Increase the number of Kafka brokers.
1. Add more inlet instances and shard the exporters among the configured ones.

The value of the receive buffer is also available as a metric:

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics | grep -P 'akvorado_inlet_flow_input_udp_buffer'
​# HELP akvorado_inlet_flow_input_udp_buffer_size_bytes Size of the in-kernel buffer for this worker.
​# TYPE akvorado_inlet_flow_input_udp_buffer_size_bytes gauge
akvorado_inlet_flow_input_udp_buffer_size_bytes{listener=":2055",worker="2"} 212992
```

## Outlet

The outlet is expected to automatically scale the number of workers to ensure
that the data is delivered efficiently to ClickHouse. Increasing the maximum
number of Kafka workers (`max-workers`) past the default value of 8 may put more
pressure on ClickHouse. However, you can increase `maximum-batch-size`.
Moreover, there cannot be more workers than the number of partitions for the
topic. This part is configurable on the orchestrator
(`kafka`→`topic-configuration`→`num-partitions`).

BMP is a one-way protocol, and the sender may declare the receiver station
“stuck” if it does not accept more data. To handle this, BMP messages are
internally buffered in a queue between the TCP reader and the message processor.
The `message-buffer` configuration key controls the size of this queue (default:
10000 messages). This decouples IO from processing and prevents session drops
during slow operations such as peer removal.

You can monitor backpressure using the `message_queue_full_total` and
`message_queue_notfull_total` metrics:

```console
# HELP akvorado_outlet_routing_provider_bmp_message_queue_full_total Number of BMP messages hitting the message queue limit.
# TYPE akvorado_outlet_routing_provider_bmp_message_queue_full_total counter
akvorado_outlet_routing_provider_bmp_message_queue_full_total{exporter="247.16.14.12"} 2
akvorado_outlet_routing_provider_bmp_message_queue_full_total{exporter="247.16.14.13"} 0
# HELP akvorado_outlet_routing_provider_bmp_message_queue_notfull_total Number of BMP messages not hitting the message queue limit.
# TYPE akvorado_outlet_routing_provider_bmp_message_queue_notfull_total counter
akvorado_outlet_routing_provider_bmp_message_queue_notfull_total{exporter="247.16.14.12"} 17998
akvorado_outlet_routing_provider_bmp_message_queue_notfull_total{exporter="247.16.14.13"} 0
```

## Profiling

On a large-scale installation, you may want to check if *Akvorado* is using too
much CPU or memory. You can do this with `pprof`, the [Go
profiler](https://go.dev/blog/pprof). You need a working [Go
installation](https://go.dev/doc/install) on your workstation.

When running on Docker, use `docker inspect` to get the IP address of the service
that you want to profile (inlet or outlet):

```console
$ docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' akvorado-akvorado-inlet-1
240.0.4.8
$ docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' akvorado-akvorado-outlet-1
240.0.4.9
```

Then, use one of these two commands:

```console
$ go tool pprof http://240.0.4.8:8080/debug/pprof/profile
$ go tool pprof http://240.0.4.8:8080/debug/pprof/heap
```

If your Docker host is remote, you also need to use SSH forwarding to expose the
HTTP port to your workstation:

```console
$ ssh -L 6060:240.0.4.8:8080 dockerhost.example.com
```

Then, use one of these two commands:

```console
$ go tool pprof http://127.0.0.1:6060/debug/pprof/profile
$ go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

The first one provides a CPU profile. The second one provides a memory profile. On the
command line, you can type `web` to visualize the result in the browser or `svg`
to get an SVG file that you can attach to a bug report if needed.
