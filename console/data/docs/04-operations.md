# Operations

While Akvorado itself does not require much memory and disk space, both Kafka
and ClickHouse have heavier needs. To get started, do not try to run the
complete setup with less than 16 GB of RAM (32 GB or more is advised) and with
less than 50 GB of disk (100 GB or more is advised). Use at least 8 vCPU.

## Router configuration

Each router should be configured to send flows to Akvorado inlet
service and accepts SNMP requests. For routers not listed below, have
a look at the [configuration
snippets](https://github.com/kentik/config-snippets/) from Kentik.

It is better to **sample on ingress only**. This requires sampling on both
external and internal interfaces, but this prevents flows from being accounted twice
when they enter and exit through external ports.

### Exporter Address

The exporter address is set from the field inside the flow message by default,
and used e.g. for SNMP requests. However, if for some reason the set flow
address (also called agent ID) is wrong, you can use the source IP of the flow
packet instead by setting `use-src-addr-for-exporter-addr: true` for the flow
configuration.

Please note that with this configuration, your deployment must not touch the
source IP! This might occur with Docker or Kubernetes networking.

### Cisco IOS-XE

NetFlow can be enabled with the following configuration:

```cisco
flow record Akvorado
    match ipv4 tos
    match ipv4 protocol
    match ipv4 source address
    match ipv4 destination address
    match transport source-port
    match transport destination-port
    collect routing source as 4-octet
    collect routing destination as 4-octet
    collect routing next-hop address ipv4
    collect transport tcp flags
    collect interface output
    collect interface input
    collect counter bytes
    collect counter packets
    collect timestamp sys-uptime first
    collect timestamp sys-uptime last
!
flow record Akvorado-IPV6
    match ipv6 protocol
    match ipv6 source address
    match ipv6 destination address
    match transport source-port
    match transport destination-port
    collect routing source as 4-octet
    collect routing destination as 4-octet
    collect routing next-hop address ipv4
    collect transport tcp flags
    collect interface output
    collect interface input
    collect counter bytes
    collect counter packets
    collect timestamp sys-uptime first
    collect timestamp sys-uptime last
!
sampler random1in100
    mode random 1 out-of 100
!
flow exporter AkvoradoExport
    destination <akvorado-ip> vrf monitoring
    source Loopback20
    transport udp 2055
    version 9
    option sampler-table timeout 10
!
flow monitor AkvoradoMonitor
    exporter AkvoradoExport
    cache timeout inactive 10
    cache timeout active 60
    record Akvorado
! 
flow monitor AkvoradoMonitor-IPV6
    exporter AkvoradoExport
    cache timeout inactive 10
    cache timeout active 60
    record Akvorado-IPV6
!
```

To enable NetFlow on an interface, use the following snippet:

```cisco
interface GigabitEthernet0/0/3
    ip flow monitor AkvoradoMonitor sampler random1in100 input
    ip flow monitor AkvoradoMonitor sampler random1in100 output
    ipv6 flow monitor AkvoradoMonitor-IPV6 sampler random1in100 input
    ipv6 flow monitor AkvoradoMonitor-IPV6 sampler random1in100 output
!
```

As per [issue #89](https://github.com/akvorado/akvorado/issues/89), the sampling
rate is not reported correctly on this platform. The solution is to set a
default sampling rate in `akvorado.yaml`. Check the
[documentation](02-configuration.html#core) for more details.

```yaml
inlet:
  core:
    default-sampling-rate: 100
```

### NCS 5500 and ASR 9000

On each router, NetFlow can be enabled with the following configuration. It is
important to use a power of two for the sampling rate (at least on NCS).

```cisco
sampler-map sampler1
 random 1 out-of 32768
!
flow exporter-map akvorado
 version v9
  options sampler-table timeout 10
  template options timeout 10
 !
 transport udp 2055
 source Loopback20
 destination <akvorado-ip> vrf private
!
flow monitor-map monitor1
 record ipv4
 exporter akvorado
 cache entries 100000
 cache timeout active 15
 cache timeout inactive 2
 cache timeout rate-limit 2000
!
flow monitor-map monitor2
 record ipv6
 exporter akvorado
 cache entries 100000
 cache timeout active 15
 cache timeout inactive 2
 cache timeout rate-limit 2000
!
```

Optionally, AS path can be pushed to the forwarding database and the
source and destination AS will be present in NetFlow packets:

```cisco
router bgp <asn>
 address-family ipv4 unicast
  bgp attribute-download
!
 address-family ipv6 unicast
  bgp attribute-download
```

To enable NetFlow on an interface, use the following snippet:

```cisco
interface Bundle-Ether4000
 flow ipv4 monitor monitor1 sampler sampler1 ingress
 flow ipv6 monitor monitor2 sampler sampler1 ingress
!
```

Also check the [troubleshooting section](05-troubleshooting.md) on how
to scale NetFlow on the NCS 5500.

Then, SNMP needs to be enabled:

```cisco
snmp-server community <community> RO IPv4
snmp-server ifindex persist
control-plane
 management-plane
  inband
   interface all
    allow SNMP peer
     address ipv4 <akvorado-ip>
```

To configure BMP, adapt the following snippet:

```cisco
bmp server 1
 host <akvorado-ip> port 10179
 flapping-delay 60
bmp server all
 route-monitoring policy post inbound
router bgp 65400
 vrf public
  neighbor 192.0.2.100
   bmp-activate server 1
```

### Juniper

#### NetFlow

For MX and SRX devices, you can use NetFlow v9 to export flows.

```junos
groups {
  sampling {
    interfaces {
      <*> {
        unit <*> {
          family inet {
            sampling {
              input;
            }
          }
          family inet6 {
            sampling {
              input;
            }
          }
        }
      }
    }
  }
}
forwarding-options {
  sampling {
    instance {
      sample-ins {
        input {
          rate 1024;
          max-packets-per-second 65535;
        }
        family inet {
          output {
            flow-server 192.0.2.1 {
              port 2055;
              autonomous-system-type origin;
              source-address 203.0.113.2;
              version9 {
                template {
                  ipv4;
                }
              }
            }
            inline-jflow {
              source-address 203.0.113.2;
            }
          }
        }
        family inet6 {
          output {
            flow-server 192.0.2.1 {
              port 2055;
              autonomous-system-type origin;
              source-address 203.0.113.2;
              version9 {
                template {
                  ipv6;
                }
              }
            }
            inline-jflow {
              source-address 203.0.113.2;
            }
          }
        }
      }
    }
  }
}
chassis {
  fpc 0 {
    sampling-instance sample-ins;
    inline-services {
      flex-flow-sizing;
    }
  }
}
services {
  flow-monitoring {
    version9 {
      template ipv4 {
        nexthop-learning enable;
        flow-active-timeout 10;
        flow-inactive-timeout 10;
        template-refresh-rate {
          packets 30;
          seconds 30;
        }
        option-refresh-rate {
          packets 30;
          seconds 30;
        }
        ipv4-template;
      }
      template ipv6 {
        nexthop-learning enable;
        flow-active-timeout 10;
        flow-inactive-timeout 10;
        template-refresh-rate {
          packets 30;
          seconds 30;
        }
        option-refresh-rate {
          packets 30;
          seconds 30;
        }
        ipv6-template;
      }
    }
  }
}
```

Then, for each interface you want to enable IPFIX on, use:

```junos
interfaces {
  xe-0/0/0.0 {
    description "Transit: Cogent AS179 [3-10109101]";
    apply-groups [ sampling ];
  }
}
```

If `inet.0` is not enough to join *Akvorado*, you need to add a specific route:

```junos
routing-options {
  static {
    route 192.0.2.1/32 next-table internet.inet.0;
  }
}
```

Another option would be IPFIX (replace `version9` by `version-ipfix`).
However, Juniper includes only *total* counters for bytes and packets
rather than using *delta* counters. *Akvorado* does not support such
counters.

#### sFlow

For QFX devices, you can use sFlow.

```junos
protocols {
    sflow {
        agent-id 203.0.113.4;
        polling-interval 5;
        sample-rate ingress 8192;
        source-ip 203.0.113.4;
        collector 192.0.2.1 {
            udp-port 6343;
        }
        interfaces et-0/0/13.0;
    }
}
```

#### SNMP

Then, configure SNMP:

```junos
snmp {
  location "Equinix PA1, FR";
  community blipblop {
    authorization read-only;
    routing-instance internet;
  }
  routing-instance-access;
}
```

#### BMP

If needed, you can configure BMP on one router to send all AdjRIB-in
to Akvorado.

```junos
routing-options {
    bmp {
        connection-mode active;
        station-address 203.0.113.1;
        station-port 10179;
        station collector;
        hold-down 30 flaps 10 period 30;
        route-monitoring post-policy;
        monitor enable;
    }
}
```

See [Juniper's documentation](https://www.juniper.net/documentation/us/en/software/junos/bgp/topics/ref/statement/bmp-edit-routing-options.html) for more details.

### Arista

#### sFlow

For Arista devices, you can use sFlow.

```eos
sflow sample 1024
sflow sample output subinterface
sflow sample input subinterface
sflow vrf VRF-MANAGEMENT destination 192.0.2.1
sflow vrf VRF-MANAGEMENT source-interface Management1
sflow interface egress enable default
sflow run
```

#### SNMP

Then, configure SNMP:

```eos
snmp-server community <community> ro
snmp-server vrf VRF-MANAGEMENT
```

### Nokia SR OS

Model-driven command line interface (MD-CLI) syntax is used below. The
full-context is provided as this is probably easier to adapt to classic CLI.

#### Flows

Flow is currently barely supported on devices running SR OS, one mostly has to
stick to IPFIX.

```
/configure cflowd admin-state enable
/configure cflowd cache-size 250000
/configure cflowd template-retransmit 60
/configure cflowd active-flow-timeout 15
/configure cflowd inactive-flow-timeout 15
/configure cflowd sample-profile 1 sample-rate 2000
/configure cflowd collector 192.0.2.1 port 2055 admin-state enable
/configure cflowd collector 192.0.2.1 port 2055 description "akvorado.example.net"
/configure cflowd collector 192.0.2.1 port 2055 router-instance "Base"
/configure cflowd collector 192.0.2.1 port 2055 version 10
```

Either configure sampling on the individual interfaces:

```
/configure service ies "internet" interface "if1/1/c1/1:0" cflowd-parameters sampling unicast type interface
/configure service ies "internet" interface "if1/1/c1/1:0" cflowd-parameters sampling unicast direction ingress-only
/configure service ies "internet" interface "if1/1/c1/1:0" cflowd-parameters sampling unicast sample-profile 1
```

Or add it to apply groups which are probably already in place:

```
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast type interface
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast direction ingress-only
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast sample-profile 1

/configure service ies "internet" interface "if1/1/c1/1:0" apply-groups ["peering"]
```

#### SNMP

Nokia routers running SR OS use a different interface index in their flow
records as the SNMP interface index usually used by other devices. To fix this
issue, you need to use `cflowd use-vrtr-if-index`. More information can be found
in [Nokia's
documentation](https://infocenter.nokia.com/public/7750SR140R4/topic/com.sr.router.config/html/cflowd_cli.html#tgardner5iexrn6muno).

#### GNMI

Instead of SNMP GNMI can be used. The interface index challenge (see `SNMP`
above) also applies. See this
[discussion](https://github.com/akvorado/akvorado/discussions/1275) for further
details and possible workarounds.

In the below example, unencrupted connections are used. Check the documentation
if you want to enable TLS for a more secure setup.

```
/configure system grpc admin-state enable
/configure system grpc allow-unsecure-connection
/configure system security user-params local-user user "akvorado" access grpc true
/configure system security user-params local-user user "akvorado" console member ["grpc_ro"]
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnmi-get permit
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnmi-set deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnmi-subscribe permit
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnoi-file-get deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnoi-file-transfertoremote deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnoi-file-put deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnoi-file-stat deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization gnoi-file-remove deny
/configure system security aaa local-profiles profile "grpc_ro" grpc rpc-authorization md-cli-session deny
```

#### BMP

```
/configure bmp admin-state enable
/configure bmp station "akvorado" admin-state enable
/configure bmp station "akvorado" description "akvorado.example.net"
/configure bmp station "akvorado" stats-report-interval 300
/configure bmp station "akvorado" connection local-address 192.0.2.42
/configure bmp station "akvorado" connection station-address ip-address 192.0.2.1
/configure bmp station "akvorado" connection station-address port 10179
/configure bmp station "akvorado" family ipv4 true
/configure bmp station "akvorado" family ipv6 true
```

```
/configure router "Base" bgp monitor admin-state enable
/configure router "Base" bgp monitor route-monitoring post-policy true
/configure router "Base" bgp monitor station "akvorado" { }
```

### GNU/Linux

#### pmacctd

Configure pmacctd with sFlow receiver:
```yaml
/etc/pmacctd/config.conf: |
  daemonize: false
  plugins: sfprobe[any]
  sfprobe_receiver: akvorado-inlet-receiver-replace-me:6343
  aggregate: src_host,dst_host,in_iface,out_iface,src_port,dst_port,proto
  pcap_ifindex: map
  pcap_interfaces_map: /etc/pmacctd/interfaces.map
  pcap_interface_wait: true
  sfprobe_agentsubid: 1402
  sampling_rate: 1000
  snaplen: 128
/etc/pmacctd/interfaces.map: |
  ifindex=1 ifname=lo direction=in
  ifindex=1 ifname=lo direction=out
  ifindex=3 ifname=eth0 direction=in
  ifindex=3 ifname=eth0 direction=out
  ifindex=4 ifname=eth1 direction=in
  ifindex=4 ifname=eth1 direction=out
```

Here we set the interface indexes manually entirely based on the interface
names and completely ignoring the kernel ifIndex for the flows. pmacctd can
be run inside containers where SNMPd does not return descriptions for the
interfaces, which is a required field for the flow. With this setup, you can
make use of the static metadata provider to match the exporter and accept the
flow for further classification.

## Kafka

When using `docker compose`, there is a Kafka UI running at
`http://127.0.0.1:8080/kafka-ui/`. It provides various operational
metrics you can check, notably the space used by each topic.

## ClickHouse

While ClickHouse works pretty well out-of-the-box, it is still
encouraged to read [its documentation](https://clickhouse.com/docs/).
Altinity also provides a [knowledge base](https://kb.altinity.com/)
with various other tips.

### System tables

ClickHouse is configured to log various events into MergeTree tables. By
default, these tables are unbounded. Unless configured otherwise, the
orchestrator sets a TTL of 30 days. These tables can also be customized in the
configuration files or disabled completely. See [ClickHouse
documentation](https://clickhouse.com/docs/en/operations/system-tables/) for
more details.

The following request is useful to see how much space is used for each
table:

```sql
SELECT database, name, formatReadableSize(total_bytes)
FROM system.tables
WHERE total_bytes > 0
ORDER BY total_bytes DESC
```

If you see tables suffixed by `_0` or `_1`, they can be deleted: they are
created when ClickHouse is updated with the data from the tables before the
upgrade.

### Memory usage

The `networks` dictionary can take a bit of memory. You can check with the following queries:

```sql
SELECT name, status, type, formatReadableSize(bytes_allocated)
FROM system.dictionaries
```

### Space usage

To get the space used by ClickHouse, use the following query:

```sql
SELECT formatReadableSize(sum(bytes_on_disk)) AS size
FROM system.parts
```

You can get an idea on how much space is used by each table with the
following query:

```sql
SELECT table, formatReadableSize(sum(bytes_on_disk)) AS size, MIN(partition_id) AS oldest
FROM system.parts
WHERE table LIKE 'flow%'
GROUP by table
```

The following query shows how much space is used by each column for the `flows`
table and how much they are compressed. This can be helpful if you find too much
space is used by this table.

```sql
SELECT
    database,
    table,
    column,
    type,
    sum(rows) AS rows,
    sum(column_data_compressed_bytes) AS compressed_bytes,
    formatReadableSize(compressed_bytes) AS compressed,
    formatReadableSize(sum(column_data_uncompressed_bytes)) AS uncompressed,
    sum(column_data_uncompressed_bytes) / compressed_bytes AS ratio,
    any(compression_codec) AS codec
FROM system.parts_columns AS pc
LEFT JOIN system.columns AS c ON (pc.database = c.database) AND (c.table = pc.table) AND (c.name = pc.column)
WHERE table = 'flows' AND active
GROUP BY
    database,
    table,
    column,
    type
ORDER BY
    database ASC,
    table ASC,
    sum(column_data_compressed_bytes) DESC
```

You can also have a look at the system tables:

```sql
SELECT * EXCEPT size, formatReadableSize(size) AS size FROM (
 SELECT database, table, sum(bytes_on_disk) AS size, MIN(partition_id) AS oldest
 FROM system.parts
 GROUP by database, table
 ORDER by size DESC
)
```

All the system tables with suffix `_0`, `_1` are tables from an older version of
ClickHouse. You can drop them by using this SQL query and copy-pasting the
result:

```sql
SELECT concat('DROP TABLE IF EXISTS system.', name, ';')
FROM system.tables
WHERE (database = 'system') AND match(name, '_[0-9]+$')
FORMAT TSVRaw
```

### CPU usage

If ClickHouse has a high CPU usage, you can extract slow queries with:

```sql
SELECT formatReadableTimeDelta(query_duration_ms/1000) AS duration, query
FROM system.query_log
WHERE query_kind = 'Select'
ORDER BY query_duration_ms DESC
LIMIT 10
FORMAT Vertical
```

Also check slow inserts:

```sql
SELECT formatReadableTimeDelta(query_duration_ms/1000) AS duration, query
FROM system.query_log
WHERE query_kind = 'Insert'
ORDER BY query_duration_ms DESC
LIMIT 10
FORMAT Vertical
```


[Altinity's knowledge
base](https://kb.altinity.com/altinity-kb-useful-queries/query_log/)
contains some other useful queries.

One cause of slowness, is that there are too many active parts:

```sql
SELECT table, count() AS parts
FROM system.parts
WHERE active AND (database = currentDatabase())
GROUP BY table
```

If some tables are over 300, it may mean the inserts are too small. In this
case, you should reduce the number of Kafka workers for the outlet until the
number of flows per batch is around 100 000 (check the
`akvorado_outlet_clickhouse_flow_per_batch` metric).

### Old tables

Tables not used anymore may still be around check with `SHOW TABLES`. You can
drop the following tables:

- `flows_raw_errors`
- `flows_raw_errors_consumer`
- any `flows_XXXXXXX_raw_errors`
- any `flows_XXXXXXX_raw` and `flows_XXXXXXX_raw_consumer` when `XXXXXXX` does not end with `vN` where `N` is a number
- any `flows_XXXXXvN_raw` and `flows_XXXXXvN_raw_consumer` when another table exists with a higher `N` value

These tables do not contain data. If you make a mistake, you can restart the orchestrator to recreate them.
