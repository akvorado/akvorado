# Operations

## Router configuration

Each router should be configured to send flows to Akvorado inlet
service and accepts SNMP requests. For routers not listed below, have
a look at the [configuration
snippets](https://github.com/kentik/config-snippets/) from Kentik.

### NCS 5500 and ASR 9000

On each router, Netflow can be enabled with the following configuration:

```cisco
sampler-map sampler1
 random 1 out-of 30000
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

Also, SNMP needs to be enabled:

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

## Kafka

When using `docker-compose`, there is a Kafka UI running at
`http://127.0.0.1:8080/kafka-ui/`. It provides various operational
metrics you can check, notably the space used by each topic.

## ClickHouse

While ClickHouse works pretty good out-of-the-box, it is still
encouraged to read [its documentation](https://clickhouse.com/docs/).
Altinity also provides a [knowledge base](https://kb.altinity.com/)
with various other tips.

### System tables

ClickHouse is configured to log various events into MergeTree tables.
By default, these tables are unbounded. You should set a TTL to avoid
them to grow indefinitely:

```sql
ALTER TABLE system.trace_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.query_thread_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.query_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.asynchronous_metric_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.metric_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.part_log MODIFY TTL event_date + INTERVAL 30 day DELETE ;
ALTER TABLE system.session_log MODIFY TTL event_date + INTERVAL 30 day DELETE
```

These tables can also be customized in the configuration files or
disabled completly. See [ClickHouse
documentation](https://clickhouse.com/docs/en/operations/system-tables/)
for more details.

The following request is useful to see how much space is used for each
table:

```sql
SELECT database, name, formatReadableSize(total_bytes)
FROM system.tables
WHERE total_bytes > 0
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
