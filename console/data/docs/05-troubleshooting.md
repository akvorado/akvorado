# Troubleshooting

> [!WARNING]
> Please read this page carefully before opening an issue or starting a discussion.

> [!TIP]
> This guide assumes you use the *Docker Compose* setup. If you use a different setup, adapt the commands as needed.

As explained in the [introduction](00-intro#big-picture), Akvorado has several
components. To troubleshoot an issue, inspect each component.

![Functional view](troubleshoot.svg)

Your routers send flows to the *inlet*, which forwards them to *Kafka*. The
*outlet* pulls flows from Kafka, decodes and processes them, then sends them to
*ClickHouse*. The *orchestrator* configures *Kafka* and *ClickHouse* and
provides configuration for the *inlet* and *outlet*. The *console* (not shown
here) queries *ClickHouse* to display flows to users.

## Basic checks

First, check you have enough space. This is a common cause of failure:

```console
$ docker system df
TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images          7         7         1.819GB   7.834MB (0%)
Containers      15        15        2.752GB   0B (0%)
Local Volumes   16        9         69.24GB   8.594GB (12%)
Build Cache     4         0         5.291MB   5.291MB
```

You can recover space with `docker system prune` or get more details with
`docker system df -v`. Have a look at the documentation about
[operations](04-operations.md#clickhouse) on how to check space usage for
ClickHouse.

> [!CAUTION]
> Do **not** use `docker system prune -a` unless you are pretty sure that all your
> containers are up and running. It is important to understand that this command
> removes anything not currently used.

Check that all components are running and healthy:

```console
$ docker compose ps --format "table {{.Service}}\t{{.Status}}"
SERVICE                    STATUS
akvorado-conntrack-fixer   Up 28 minutes
akvorado-console           Up 27 minutes (healthy)
akvorado-inlet             Up 27 minutes (healthy)
akvorado-orchestrator      Up 27 minutes (healthy)
akvorado-outlet            Up 27 minutes (healthy)
clickhouse                 Up 28 minutes (healthy)
geoip                      Up 28 minutes (healthy)
kafka                      Up 28 minutes (healthy)
kafka-ui                   Up 28 minutes
redis                      Up 28 minutes (healthy)
traefik                    Up 28 minutes
```

Make sure all components are present. If a component is missing, restarting,
unhealthy, or not working properly, check its logs:

```console
$ docker compose logs akvorado-inlet
```

The *inlet*, *outlet*, *orchestrator*, and *console* expose metrics. Get them with this command:

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics
​# HELP akvorado_cmd_info Akvorado build information
​# TYPE akvorado_cmd_info gauge
akvorado_cmd_info{compiler="go1.24.4",version="v1.11.5-134-gaf3869cd701c"} 1
[...]
```

> [!CAUTION]
> Run the `curl` command on the same host that runs Akvorado, and change `inlet`
> with the component name you are interested in.

To see only error metrics, filter them:

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics | grep 'akvorado_.*_error'
```

> [!TIP]
> To follow this guide on a working system, replace `http://127.0.0.1:8080` with `https://demo.akvorado.net`.

### Inlet service

The inlet service receives NetFlow/IPFIX/sFlow packets and forwards them to
Kafka. First, check if you receive packets from exporters (your routers):

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics | grep 'akvorado_inlet_flow_input_udp_packets'
​# HELP akvorado_inlet_flow_input_udp_packets_total Packets received by the application.
​# TYPE akvorado_inlet_flow_input_udp_packets_total counter
akvorado_inlet_flow_input_udp_packets_total{exporter="241.107.1.12",listener=":2055",worker="2"} 6769
akvorado_inlet_flow_input_udp_packets_total{exporter="241.107.1.13",listener=":2055",worker="1"} 6794
akvorado_inlet_flow_input_udp_packets_total{exporter="241.107.1.14",listener=":2055",worker="2"} 6765
akvorado_inlet_flow_input_udp_packets_total{exporter="241.107.1.15",listener=":2055",worker="0"} 6782
```

If your exporters are not listed, check their configuration. You can also use
`tcpdump` to verify they are sending packets. Replace the IP by the IP address
of the exporter and the port by the right port (2055 for NetFlow and 6343 for
sFlow).

```console
# tcpdump -c3 -pni any host 241.107.1.12 and port 2055
09:11:08.729738 IP 241.107.1.12.44026 > 240.0.2.9.2055: UDP, length 624
09:11:08.729787 IP 241.107.1.12.44026 > 240.0.2.9.2055: UDP, length 1060
09:11:08.729799 IP 241.107.1.12.44026 > 240.0.2.9.2055: UDP, length 1060
3 packets captured
3 packets received by filter
0 packets dropped by kernel
```

Next, check if flows are sent to Kafka correctly:

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics | grep 'akvorado_inlet_kafka_sent_messages'
​# HELP akvorado_inlet_kafka_sent_messages_total Number of messages sent from a given exporter.
​# TYPE akvorado_inlet_kafka_sent_messages_total counter
akvorado_inlet_kafka_sent_messages_total{exporter="241.107.1.12"} 8108
akvorado_inlet_kafka_sent_messages_total{exporter="241.107.1.13"} 8117
akvorado_inlet_kafka_sent_messages_total{exporter="241.107.1.14"} 8090
akvorado_inlet_kafka_sent_messages_total{exporter="241.107.1.15"} 8123
```

If no messages appear here, there may be a Kafka problem.

### Kafka

The *inlet* is sending messages to Kafka and the *outlet* is pulling them from
Kafka. The Docker Compose setup is shipped with [UI for Apache
Kafka](https://github.com/provectus/kafka-ui). You can reach it through
`http://127.0.0.1:8080/kafka-ui`.

> [!TIP]
> For security reasons, this UI is not exposed on anything else than the host
> running Akvorado. If you need to access it remotely, the easiest way would be
> to use [SSH port
> forwarding](https://www.digitalocean.com/community/tutorials/ssh-port-forwarding):
> `ssh -L 8080:127.0.0.1:8080 akvorado`. Then, you can use
> `http://127.0.0.1:8080/kafka-ui` directly from your workstation.

Check the various tabs (brokers, topics, and consumers) to check everything is
green. In “brokers”, you should see one broker. In “topics”, you should see
`flows-v5` with an increasing number of messages. This means the *inlet* is
pushing messages. In “consumers”, you should have `akvorado-outlet`, with at
least one member. The consumer lag should be stable (and low): this is the
number of messages that have not been processed by the *outlet* yet.

### Outlet

The *outlet* is the most complex component. Check if it works correctly with
this command (should show one processed flow):

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/flows\?limit\=1
{"TimeReceived":1753631373,"SamplingRate":100000,"ExporterAddress":"::ffff:241.107.1.15","InIf":10,"OutIf":21,"SrcVlan":0,"DstVlan":0,"SrcAddr":"::ffff:216.58.206.244","DstAddr":"::ffff:192.0.2.144","NextHop":"","SrcAS":15169,"DstAS":64501,"SrcNetMask":24,"DstNetMask":24,"OtherColumns":null}
```

Check these important metrics. First, the outlet should receive flows from
Kafka:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/metrics | grep 'akvorado_outlet_kafka_received_messages'
​# HELP akvorado_outlet_kafka_received_messages_total Number of messages received for a given worker.
​# TYPE akvorado_outlet_kafka_received_messages_total counter
akvorado_outlet_kafka_received_messages_total{worker="0"} 5561
akvorado_outlet_kafka_received_messages_total{worker="1"} 5456
akvorado_outlet_kafka_received_messages_total{worker="2"} 5583
akvorado_outlet_kafka_received_messages_total{worker="3"} 11068
akvorado_outlet_kafka_received_messages_total{worker="4"} 11151
akvorado_outlet_kafka_received_messages_total{worker="5"} 5588
```

If these numbers are not increasing, there is a problem receiving from Kafka. If
everything is OK, check if the flow processing pipeline works correctly:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/metrics | grep -P 'akvorado_outlet_core_(received|forwarded)'
​# HELP akvorado_outlet_core_forwarded_flows_total Number of flows forwarded to Kafka.
​# TYPE akvorado_outlet_core_forwarded_flows_total counter
akvorado_outlet_core_forwarded_flows_total{exporter="241.107.1.12"} 182512
akvorado_outlet_core_forwarded_flows_total{exporter="241.107.1.13"} 182366
akvorado_outlet_core_forwarded_flows_total{exporter="241.107.1.14"} 182278
akvorado_outlet_core_forwarded_flows_total{exporter="241.107.1.15"} 182900
​# HELP akvorado_outlet_core_received_flows_total Number of incoming flows.
​# TYPE akvorado_outlet_core_received_flows_total counter
akvorado_outlet_core_received_flows_total{exporter="241.107.1.12"} 182512
akvorado_outlet_core_received_flows_total{exporter="241.107.1.13"} 182366
akvorado_outlet_core_received_flows_total{exporter="241.107.1.14"} 182278
akvorado_outlet_core_received_flows_total{exporter="241.107.1.15"} 182900
​# HELP akvorado_outlet_core_received_raw_flows_total Number of incoming raw flows (proto).
​# TYPE akvorado_outlet_core_received_raw_flows_total counter
akvorado_outlet_core_received_raw_flows_total 45812
```

Notably, `akvorado_outlet_core_received_raw_flows_total` is incremented by one
for each message received from Kafka. The message is then decoded and the flows
are extracted. For each extracted flow,
`akvorado_outlet_core_received_flows_total` is incremented by one. The flows are
then enriched and before forwarding them to ClickHouse,
`akvorado_outlet_core_forwarded_flows_total` is incremented.

If `akvorado_outlet_core_received_raw_flows_total` increases but
`akvorado_outlet_core_received_flows_total` does not, there is an error
**decoding the flows**. If `akvorado_outlet_core_received_flows_total` increases
but `akvorado_outlet_core_forwarded_flows_total` does not, there is an error
**enriching the flows**.

For the first case, use this command to find clues:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/metrics | grep 'akvorado_outlet_flow.*errors'
```

For the second case, use this one:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/metrics | grep 'akvorado_outlet_core.*errors'
```

Here is a list of errors you may find:

- `metadata cache miss` means interface information is missing from the metadata
  cache. The most likely cause is that the exporter does not accept SNMP
  requests or the SNMP community is configured incorrectly.
- `sampling rate missing` means sampling rate information is not present. This
  is normal when Akvorado starts but should not keep increasing. With NetFlow,
  the sampling rate is sent in an options data packet. Make sure your exporter
  sends them (look for `sampler-table` in the documentation). Alternatively,
  configure `outlet`→`core`→`default-sampling-rate` to work around this issue.
- `input and output interfaces missing` means the flow does not contain input
  and output interface indexes. Fix this on the exporter.

To check the SNMP configuration is correct, a convenient way is to use
`tcpdump`.

```console
# tcpdump -c3 -pni any port 161
20:46:44.812243 IP 240.0.2.11.34554 > 240.0.2.13.161: C="private" GetRequest(95) .1.3.6.1.2.1.1.5.0 .1.3.6.1.2.1.2.2.1.2.11 .1.3.6.1.2.1.31.1.1.1.1.11 .1.3.6.1.2.1.31.1.1.1.18.11 .1.3.6.1.2.1.31.1.1.1.15.11
20:46:45.144567 IP 240.0.2.13.161 > 240.0.2.11.34554: C="private" GetResponse(153) .1.3.6.1.2.1.1.5.0="dc3-edge1.example.com" .1.3.6.1.2.1.2.2.1.2.11="Gi0/0/0/11" .1.3.6.1.2.1.31.1.1.1.1.11="Gi0/0/0/11" .1.3.6.1.2.1.31.1.1.1.18.11="Transit: Lumen" .1.3.6.1.2.1.31.1.1.1.15.11=10000
^C
2 packets captured
2 packets received by filter
0 packets dropped by kernel
```

If you don't get an answer, there may be several causes:

- the community is incorrect and you need to fix it
- the exporter is not configured to answer to SNMP requests

Finally, check if flows are sent to ClickHouse successfully. Use this command:

```
$ curl -s http://127.0.0.1:8080/api/v0/outlet/metrics | grep -P 'akvorado_outlet_clickhouse_(errors|flow)'
# HELP akvorado_outlet_clickhouse_errors_total Errors while inserting into ClickHouse
# TYPE akvorado_outlet_clickhouse_errors_total counter
akvorado_outlet_clickhouse_errors_total{error="send"} 7
​# HELP akvorado_outlet_clickhouse_flow_per_batch Number of flow per batch sent to ClickHouse
​# TYPE akvorado_outlet_clickhouse_flow_per_batch summary
akvorado_outlet_clickhouse_flow_per_batch{quantile="0.5"} 250
akvorado_outlet_clickhouse_flow_per_batch{quantile="0.9"} 480
akvorado_outlet_clickhouse_flow_per_batch{quantile="0.99"} 950
akvorado_outlet_clickhouse_flow_per_batch_sum 45892
akvorado_outlet_clickhouse_flow_per_batch_count 163
```

If the errors are not increasing and the `flow_per_batch_sum` is increasing,
everything works correctly.

### ClickHouse

The last component to check is ClickHouse. Connect to it with this command:

```console
$ docker compose exec clickhouse clickhouse-client
```

First, check if all the tables are present:

```console
$ SHOW TABLES
    ┌─name────────────────────────────────────────────┐
 1. │ asns                                            │
 2. │ exporters                                       │
 3. │ exporters_consumer                              │
 4. │ flows                                           │
 5. │ flows_1h0m0s                                    │
 6. │ flows_1h0m0s_consumer                           │
 7. │ flows_1m0s                                      │
 8. │ flows_1m0s_consumer                             │
 9. │ flows_5m0s                                      │
10. │ flows_5m0s_consumer                             │
11. │ flows_I6D3KDQCRUBCNCGF4BSOWTRMVIv5_raw          │
12. │ flows_I6D3KDQCRUBCNCGF4BSOWTRMVIv5_raw_consumer │
13. │ icmp                                            │
14. │ networks                                        │
15. │ protocols                                       │
16. │ tcp                                             │
17. │ udp                                             │
    └─────────────────────────────────────────────────┘
```

Check if the various dictionaries are populated:

```console
$ SELECT name, element_count FROM system.dictionaries
   ┌─name──────┬─element_count─┐
1. │ networks  │       5963224 │
2. │ udp       │          5495 │
3. │ icmp      │            58 │
4. │ protocols │           129 │
5. │ asns      │         99598 │
6. │ tcp       │          5883 │
   └───────────┴───────────────┘
```

If you have not used the console yet, some dictionaries may be empty.

To check if ClickHouse is behind, use this SQL query with `clickhouse client` to
get the lag in seconds:

```sql
SELECT (now()-max(TimeReceived))/60
FROM flows
```

If you still have problems, check the errors reported by ClickHouse:

```sql
SELECT last_error_time, last_error_message
FROM system.errors
ORDER BY last_error_time LIMIT 10
FORMAT Vertical
```

### Console

The most common console problems are empty widgets or no flows shown in the
“visualize” tab. Both problems indicate that interface classification is not
working correctly.

Interface classification marks interfaces as either “internal” or “external”. If
you have not configured interface classification, see the [configuration
guide](02-configuration.md#classification). This step is required.

## Scaling

Various bottlenecks can cause dropped packets. This is problematic because the
reported sampling rate becomes incorrect and you cannot reliably calculate the
number of bytes and packets. Both the exporters and the inlet need to be tuned
for this kind of problem.

The outlet can also be a bottleneck. In this case, the flows may appear on the
console with a delay.

### Exporters

The first problem may come from the exporter dropping flows. Usually, counters
can detect this situation, and you can solve it by reducing the exporter rate.

#### NCS5500 routers

[NetFlow, Sampling-Interval and the Mythical Internet Packet Size][1] contains
many information about the limit of this platform. The first bottleneck is a 133
Mbps shaper between an NPU and the LC CPU for the sampled packets (144 bytes
each). For example, on a NC55-36X100G line card, there are 6 NPU, each one
managing 6 ports. If we consider an average packet size of 1000, the maximum
sampling rate when all ports are full is 1:700 (formula is `Total-BW / (
Avg-Pkt-Size x 133Mbps ) x ( 144 x 8 )`).

[1]: https://xrdocs.io/ncs5500/tutorials/2018-02-19-netflow-sampling-interval-and-the-mythical-internet-packet-size/

It is possible to check if there are drops with `sh controllers npu
stats voq base 24 instance 0 location 0/0/CPU0` and looking at the
`COS2` line.

The second bottleneck is the size of the flow cache. If too small, it may
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

When this happens, either the `cache timeout rate-limit` should be increased or
the `cache entries` directive should be increased. The latter value can be
increased to 1 million per monitor-map.

#### Other routers

Other routers are likely to share the same limitations. It should be noted that
sFlow and IPFIX 315 do not have a flow cache and therefore are less likely to
have scaling problems.

### Inlet

When the inlet has scaling issues, the kernel receive buffers may drop packets.
Each listening queue has a fixed amount of receive buffers (212992 bytes by
default) to keep packets before handling them to the application. When this
buffer is full, packets are dropped.

*Akvorado* reports the number of drops for each listening socket with the
`akvorado_inlet_flow_input_udp_in_dropped_packets_total` counter. This should be
compared to `akvorado_inlet_flow_input_udp_packets_total`. Another way to get
the same information is by using `ss -lunepm` and look at the drop counter:

```console
$ nsenter -t $(pidof akvorado) -n ss -lunepm
State            Recv-Q           Send-Q                       Local Address:Port                        Peer Address:Port           Process
UNCONN           0                0                                        *:2055                                   *:*               users:(("akvorado",pid=2710961,fd=16)) ino:67643151 sk:89c v6only:0 <->
         skmem:(r0,rb212992,t0,tb212992,f4096,w0,o0,bl0,d486525)
```

In the example above, there were 486525 drops. This can be solved in three ways:

- increase the number of workers for the UDP input,
- increase the value of `net.core.rmem_max` sysctl and increase the
  `receive-buffer` setting attached to the input,
- add more inlet instances and shard the exporters among the configured ones.

### Outlet

The outlet is expects to automatically scale the number of workers to ensure the
data is delivered efficiently to ClickHouse. Increasing the maximum number of
Kafka workers (`max-workers`) past the default value of 8 may put more pressure
on ClickHouse. You can however increase `maximum-batch-size`.

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
