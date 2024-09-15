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

It is better to **sample on ingress only**. This requires to sample on both
external and internal interfaces, but this prevents flow to be accounted twice
when they enter and exit through external ports.

### Exporter Address

The exporter address is set from the field inside the flow message by default,
and used e.g. for SNMP requests. However, if for some reasons the set flow
address (also called agent id) is wrong, you can use the source IP of the flow
packet instead by setting `use-src-addr-for-exporter-addr: true` for the flow
configuration.

Please note that with this configuration, your deployment must not touch the
source IP! This might occur with Docker or Kubernetes networking.

### Cisco IOS-XE

Netflow can be enabled with the following configuration:

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
```

To enable Netflow on an interface, use the following snippet:

```cisco
interface GigabitEthernet0/0/3
 ip flow monitor AkvoradoMonitor sampler random1in100 input
 ip flow monitor AkvoradoMonitor sampler random1in100 output
!
```

As per [issue #89](https://github.com/akvorado/akvorado/issues/89), the sample
rate is not reported correctly on this platform. The solution is to set a
default sample rate in `akvorado.yaml`. Check the
[documentation](02-configuration.html#core) for more details.

```yaml
inlet:
  core:
    default-sampling-rate: 100
```

### NCS 5500 and ASR 9000

On each router, Netflow can be enabled with the following configuration. It is
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
source and destination AS will be present in Netflow packets:

```cisco
router bgp <asn>
 address-family ipv4 unicast
  bgp attribute-download
!
 address-family ipv6 unicast
  bgp attribute-download
```

To enable Netflow on an interface, use the following snippet:

```cisco
interface Bundle-Ether4000
 flow ipv4 monitor monitor1 sampler sampler1 ingress
 flow ipv6 monitor monitor2 sampler sampler1 ingress
!
```

Also check the [troubleshooting section](05-troubleshooting.md) on how
to scale Netflow on the NCS 5500.

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

#### Netflow

For MX and SRX devices, you can use Netflow v9 to export flows.

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

### Nokia

Nokia routers running SROS use a different interface index in their flow records
as the SNMP interface index usually used by other devices. To fix this issue,
you need to use `cflowd use-vrtr-if-index`. More information can be found in
[Nokia's
documentation](https://infocenter.nokia.com/public/7750SR140R4/topic/com.sr.router.config/html/cflowd_cli.html#tgardner5iexrn6muno)

## Kafka

When using `docker compose`, there is a Kafka UI running at
`http://127.0.0.1:8080/kafka-ui/`. It provides various operational
metrics you can check, notably the space used by each topic.

## ClickHouse

While ClickHouse works pretty good out-of-the-box, it is still
encouraged to read [its documentation](https://clickhouse.com/docs/).
Altinity also provides a [knowledge base](https://kb.altinity.com/)
with various other tips.

### System tables

ClickHouse is configured to log various events into MergeTree tables. By
default, these tables are unbounded. Unless configured otherwise, the
orchestrator sets a TTL of 30 days. These tables can also be customized in the
configuration files or disabled completly. See [ClickHouse
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

You can get an idea on how much space is used by each table with the
following query:

```sql
SELECT table, formatReadableSize(sum(bytes_on_disk)) AS size, MIN(partition_id) AS oldest
FROM system.parts
WHERE table LIKE 'flow%'
GROUP by table
```

The following query shows how much space is eaten by each column for the `flows`
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

### Slow queries

You can extract slow queries with:

```sql
SELECT formatReadableTimeDelta(query_duration_ms/1000) AS duration, query
FROM system.query_log
WHERE query_kind = 'Select'
ORDER BY query_duration_ms DESC
LIMIT 10
FORMAT Vertical
```

[Altinity's knowledge
base](https://kb.altinity.com/altinity-kb-useful-queries/query_log/)
contains some other useful queries.
