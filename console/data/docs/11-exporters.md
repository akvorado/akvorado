# Configure your exporters

An exporter is a device that sends flows to *Akvorado*. This page collects the
configuration snippets for the platforms we know about. Each exporter should be
configured to send flows to the inlet service and to accept SNMP requests. For a
device that is not listed here, look at the [configuration
snippets](https://github.com/kentik/config-snippets/) from Kentik.

Two settings are important on every platform:

- It is better to sample on ingress only. This requires sampling on both
  external and internal interfaces. This prevents flows from being counted twice
  when they enter and exit through external ports.
- With NetFlow or IPFIX, use low timeout values for both active and inactive
  flows. Values of 5 to 10 seconds should be OK. Higher values create spikes in
  the graphs.

After configuring a device, check that its flows are received with the
[troubleshooting guide](12-troubleshooting.md#inlet-service).

## Exporter address

The exporter address is set from the field inside the flow message by default
and is used, for example, for SNMP requests. However, if the set flow address
(also called agent ID) is wrong, you can use the source IP of the flow packet
instead by setting `use-src-addr-for-exporter-addr: true` in the flow
configuration.

Please note that with this configuration, your deployment must not change the
source IP. This might happen with Docker or Kubernetes networking.

## Cisco IOS-XE

You can enable NetFlow with the following configuration:

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
    cache timeout active 10
    record Akvorado
! 
flow monitor AkvoradoMonitor-IPV6
    exporter AkvoradoExport
    cache timeout inactive 10
    cache timeout active 10
    record Akvorado-IPV6
!
```

To enable NetFlow on an interface, use this snippet:

```cisco
interface GigabitEthernet0/0/3
    ip flow monitor AkvoradoMonitor sampler random1in100 input
    ip flow monitor AkvoradoMonitor sampler random1in100 output
    ipv6 flow monitor AkvoradoMonitor-IPV6 sampler random1in100 input
    ipv6 flow monitor AkvoradoMonitor-IPV6 sampler random1in100 output
!
```

According to [issue #89](https://github.com/akvorado/akvorado/issues/89), the
sampling rate is not reported correctly on this platform. The solution is to set
a default sampling rate in `akvorado.yaml`. See the
[documentation](50-configuration.md#core) for more details.

```yaml
inlet:
  core:
    default-sampling-rate: 100
```

## Cisco NCS 5500 and ASR 9000

On each router, you can enable NetFlow with the following configuration. It is
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
 cache timeout active 10
 cache timeout inactive 10
 cache timeout rate-limit 2000
!
flow monitor-map monitor2
 record ipv6
 exporter akvorado
 cache entries 100000
 cache timeout active 10
 cache timeout inactive 10
 cache timeout rate-limit 2000
!
```

Optionally, you can push the AS path to the forwarding database, and the source
and destination AS will be present in NetFlow packets:

```cisco
router bgp <asn>
 address-family ipv4 unicast
  bgp attribute-download
!
 address-family ipv6 unicast
  bgp attribute-download
```

To enable NetFlow on an interface, use this snippet:

```cisco
interface Bundle-Ether4000
 flow ipv4 monitor monitor1 sampler sampler1 ingress
 flow ipv6 monitor monitor2 sampler sampler1 ingress
!
```

Also, see the [scaling guide](14-scaling.md#ncs5500-routers) on how to scale
NetFlow on the NCS 5500.

Then, you need to enable SNMP:

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

To configure BMP, adapt this snippet:

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

## Juniper

### NetFlow

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

Then, for each interface you want to enable IPFIX on, use this:

```junos
interfaces {
  xe-0/0/0.0 {
    description "Transit: Cogent AS179 [3-10109101]";
    apply-groups [ sampling ];
  }
}
```

If `inet.0` is not enough to reach *Akvorado*, you need to add a specific route:

```junos
routing-options {
  static {
    route 192.0.2.1/32 next-table internet.inet.0;
  }
}
```

### IPFIX 315

Another option is using IPFIX 315. Juniper calls that inline monitoring. In this
case, Akvorado will decode the sampled packets. This is more lightweight.

```junos
services {
  inline-monitoring {
    template im-template {
      template-refresh-rate 30;
      option-template-refresh-rate 30;
      primary-data-record-fields {
        cpid-forwarding-exception-code;
        egress-interface-snmp-id; 
        ingress-interface-snmp-id;
        direction;
        datalink-frame-size;
      }
    }
    instance im-instance {
      template-name im-template {
        maximum-clip-length 126;
      }
      collector akvorado {
        source-address 203.0.113.2;
        destination-address 192.0.2.1;
        destination-port 2055;
        sampling-rate 1024;
      }
    }
  }
}
firewall {
  family inet {
    filter monitoring {
      term 1 {
        then {
          inline-monitoring-instance im-instance;
          accept;
        }
      }
    }
  }
  family inet6 {
    filter monitoring {
      term 1 {
        then {
          inline-monitoring-instance im-instance;
          accept;
        }
      }
    }
  }
}
groups {
  sampling {
    interfaces {
      <*> {
        unit <*> {
          family inet {
            filter {
              input monitoring;
            }
          }
          family inet6 {
            filter {
              input monitoring;
            }
          }
        }
      }
    }
  }
}
```

Then, for each interface you want to enable IPFIX on, use this:

```junos
interfaces {
  xe-0/0/0.0 {
    description "Transit: Cogent AS179 [3-10109101]";
    apply-groups [ sampling ];
  }
}
```

### sFlow

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

### SNMP

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

### BMP

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

## Arista

### sFlow

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

### SNMP

Then, configure SNMP:

```eos
snmp-server community <community> ro
snmp-server vrf VRF-MANAGEMENT
```

### BMP

If needed, you can also configure BMP.

```eos
router bgp 65001
   bgp monitoring
   monitoring station COLLECTOR
      update-source Management1
      connection address 10.122.4.51
      connection mode active port 10179
      export-policy received routes post-policy
      export-policy bgp rib bestpaths
```

## Nokia SR OS

The syntax below is for the model-driven command line interface (MD-CLI). The
full context is provided to make it easier to adapt to the classic CLI.

### Flows

sFlow is not well supported on devices running SR OS. It is best to use IPFIX.

```
/configure cflowd admin-state enable
/configure cflowd cache-size 250000
/configure cflowd template-retransmit 60
/configure cflowd active-flow-timeout 10
/configure cflowd inactive-flow-timeout 10
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

Or, add it to apply-groups that are probably already in place:

```
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast type interface
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast direction ingress-only
/configure groups group "peering" service ies "internet" interface "<i.*>" cflowd-parameters sampling unicast sample-profile 1

/configure service ies "internet" interface "if1/1/c1/1:0" apply-groups ["peering"]
```

### SNMP

Nokia routers running SR OS use a different interface index in their flow
records than the SNMP interface index that is usually used by other devices. To
fix this, you need to use `cflowd use-vrtr-if-index`. You can find more
information in [Nokia's
documentation](https://infocenter.nokia.com/public/7750SR140R4/topic/com.sr.router.config/html/cflowd_cli.html#tgardner5iexrn6muno).

### gNMI

Instead of SNMP, you can use gNMI. The interface index challenge (see `SNMP`
above) also applies. See this
[discussion](https://github.com/akvorado/akvorado/discussions/1275) for more
details and possible workarounds.

In the example below, unencrypted connections are used. Check the documentation
to enable TLS for a more secure setup.

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

### BMP

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

## MikroTik

For MikroTik, if you use RouterOS v6, the sampling rate is incorrectly reported
and you need to override the sampling rate in the outlet configuration:

```yaml
core:
  override-sampling-rate:
    192.168.10.10: 10000 # mikrotik1
    192.168.10.11: 2000  # mikrotik2
```

This should not be needed for RouterOS v7 (at least from version 7.10).

Here are a few resources from MikroTik help site to configure a Mikrotik device:

- [Traffic Flow](https://help.mikrotik.com/docs/spaces/ROS/pages/21102653/Traffic+Flow)
- [SNMP](https://help.mikrotik.com/docs/spaces/ROS/pages/8978519/SNMP)

## Huawei

Huawei routers support NetStream, a protocol compatible with NetFlow v9.

```text
system
ip netstream export version 9 origin-as
ip netstream export index-switch 32
ip netstream as-mode 32
ip netstream mpls-aware label-and-ip
ip netstream timeout active 10
ip netstream timeout inactive 10
ip netstream export template timeout-rate 30
ip netstream export template option sampler
ip netstream export template option timeout-rate 30
ip netstream export template option refresh-rate 30
ip netstream sampler fix-packets 2000 inbound
ip netstream sampler fix-packets 2000 outbound
ip netstream export source X.X.X.X
ip netstream export host Y.Y.Y.Y 2055
ipv6 netstream export version 9 origin-as
ipv6 netstream export index-switch 32
ipv6 netstream as-mode 32
ipv6 netstream mpls-aware label-and-ip
ipv6 netstream timeout active 10
ipv6 netstream timeout inactive 10
ipv6 netstream export template timeout-rate 30
ipv6 netstream export template option sampler
ipv6 netstream export template option timeout-rate 30
ipv6 netstream export template option refresh-rate 30
ipv6 netstream sampler fix-packets 2000 inbound
ipv6 netstream sampler fix-packets 2000 outbound
ipv6 netstream export source X.X.X.X
ipv6 netstream export host Y.Y.Y.Y 2055
```

On each interface:

```text
interface Eth-Trunk8.545
ip netstream inbound
ipv6 netstream inbound
```

On each slot:

```text
slot 1:ip netstream sampler to slot self
slot 1:ipv6 netstream sampler to slot self
```

## GNU/Linux

### pmacct

[pmacct](http://www.pmacct.net/) is a set of multi-purpose passive network
monitoring tools, including an sFlow exporter.

Put the following configuration in `/etc/pmacctd/config.conf`. Replace
`akvorado-inlet-receiver` and `sfprobe_agentip` with the correct IP.

```yaml
daemonize: false
plugins: sfprobe[any]
sfprobe_receiver: akvorado-inlet-receiver:6343
aggregate: src_host,dst_host,in_iface,out_iface,src_port,dst_port,proto
pcap_ifindex: map
pcap_interfaces_map: /etc/pmacctd/interfaces.map
pcap_interface_wait: true
sfprobe_agentsubid: 1402
sfprobe_agentip: 10.25.0.1
sampling_rate: 1000
snaplen: 128
```

In `/etc/pmacctd/interfaces.map`, adapt this snippet to your setup:

```ini
ifindex=1 ifname=lo direction=in
ifindex=1 ifname=lo direction=out
ifindex=3 ifname=eth0 direction=in
ifindex=3 ifname=eth0 direction=out
ifindex=4 ifname=eth1 direction=in
ifindex=4 ifname=eth1 direction=out
```

We set the interface indexes manually based on the interface names to
avoid running an SNMP daemon. Use the static metadata provider to match the
exporter and provide interface names and descriptions to Akvorado:

```yaml
outlet:
  metadata:
    providers:
      - type: static
        exporters:
          2001:db8:1::1:
            name: exporter1
            ifindexes:
              3:
                name: eth0
                description: PNI Google
                speed: 10000
              4:
                name: eth1
                description: PNI Netflix
                speed: 10000
```

### ipfixprobe

[ipfixprobe](https://ipfixprobe.cesnet.cz/) is a modular IPFIX flow exporter. It
supports a `pcap` plugin for low bandwidth use (less than 1 Gbps) and a
`dpdk` plugin for 100 Gbps or more.

Here is an example of how to invoke the `pcap` plugin:

```sh
ipfixprobe \
  -i "pcap;ifc=eth0;snaplen=128" \
  -s "cache;active=10;inactive=10" \
  -o "ipfix;host=akvorado-inlet-receiver;port=2055;udp;id=1;dir=1"
```

You need to run one `ipfixprobe` instance for each interface. Each interface
should have its own `id` and `dir`. As with *pmacct*, use the static metadata
provider to provide interface names and descriptions to Akvorado.

By default, ipfixprobe utilises bidirectional flows (RFC 5103) which are
supported by Akvorado.

> [!WARNING]
> The `split` option for the cache plugin results to incorrect input interfaces
> for outgoing flows.

