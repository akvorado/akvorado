# Collect your first flows

In this tutorial, you install *Akvorado* on your own server and connect one
router to it. At the end, the graphs you learned to read in the [first
tutorial](01-explore.md) show your own traffic.

Count about half an hour, most of it waiting for the containers to start.

Before you begin, you need:

- a Linux machine with [Docker](https://docs.docker.com/engine/install/) Engine
  v23 or later and [Docker Compose
  v2](https://docs.docker.com/compose/install/), with at least 16 GB of RAM and
  50 GB of free disk (see the [requirements](10-install.md#requirements)),
- one router you can configure, able to export NetFlow, IPFIX or sFlow, and to
  answer SNMP requests coming from that machine.

On Debian, the package to install is `docker-compose`. On Ubuntu, it is
`docker-compose-v2`. On macOS, install the `docker-compose` formula from
Homebrew.

> [!TIP]
> Check your version of Docker with `docker version -f '{{ .Server.Version }}'`.
> Anything older than v23 fails in ways that are hard to diagnose.

## Start Akvorado

Download the *Docker Compose* setup and start it:

```console
# mkdir akvorado
# cd akvorado
# curl -sL https://github.com/akvorado/akvorado/releases/latest/download/docker-compose-quickstart.tar.gz | tar zxvf -
# docker compose up --wait
```

The first start takes a few minutes: the images are downloaded and started. The
command returns when every service is healthy.

Open `http://127.0.0.1:8081` in a browser (or replace `127.0.0.1` with the IP
address of your server). You should get the same interface as the demo site,
with empty graphs. This is expected: no router sends anything yet.

## Let Akvorado talk to your router

Flows contain interface indexes, not interface names. *Akvorado* polls your
router with SNMP to translate them. Open `config/outlet.yaml` and set your read
community:

```yaml
metadata:
  providers:
    - type: snmp
      credentials:
        ::/0:
          communities: your-community
```

`::/0` means “every exporter”. If your routers use different communities, use
one entry per subnet instead. SNMPv3 is also supported, with the keys described
in the [configuration reference](50-configuration.md#snmp-provider).

## Describe your interfaces

This step decides whether the console shows anything at all, so do not skip it.

*Akvorado* needs to know which interfaces face the outside world. It marks each
one as external or internal, and by default the console only counts the traffic
entering through an external interface. Without this, the home page and the
graphs will be empty.

The rules are in the same file, under `core`. The shipped ones expect interface
descriptions that start with the type of connection, like `Transit: Cogent` or
`IX: FranceIX`:

```yaml
core:
  interface-classifiers:
    - |
      ClassifyConnectivityRegex(Interface.Description, "^(?i)(transit|pni|ppni|ix):? ", "$1") &&
      ClassifyProviderRegex(Interface.Description, "^\\S+?\\s(\\S+)", "$1") &&
      ClassifyExternal()
    - ClassifyInternal()
```

If your descriptions follow another convention, you need to adapt the regular
expressions. The first one match `transit`, `pni`, `ppni`, `ix` regardless of
case. So your interface descriptions should start with one of these. The second
regular expression skip the first word and use the second word as the provider
name. If both succeed, the interface is classified as external. Any interface
failing these checks get classified as internal.

Rules are tried in order. Once an interface is classified, a later rule cannot
change it, so the catch-all rule goes last. The [configuration
reference](50-configuration.md#classification) describes the complete language.

To write your own, start from the descriptions your router advertises. On most
equipment, these are the `ifAlias` values:

```console
$ snmpwalk -v 2c -c your-community 203.0.113.1 ifAlias
IF-MIB::ifAlias.10 = STRING: Transit: Cogent 1-3834938493
IF-MIB::ifAlias.11 = STRING: PNI: Netflix (WL6-1190)
IF-MIB::ifAlias.20 = STRING: core
```

Some routers do not implement `ifAlias`. *Akvorado* then uses `ifDescr`, so walk
that one instead to see what the rules receive.

Copy one of them into [regex101](https://regex101.com/), and build your pattern
there. Set the flavor to “Golang” in the left panel: *Akvorado* uses the [Go
syntax](https://github.com/google/re2/wiki/Syntax). Put the value you want to
keep inside a group, then select the “List” function and type `$1` in its box.
You get one line per description, holding what the third argument of the rule
computes. This is the value stored in `InIfConnectivity` or in `InIfProvider`,
after being converted to lower case.

The two rules above are ready to be edited, [the connectivity
one](https://regex101.com/r/KeBpV7/2/list-substitution) and [the provider
one](https://regex101.com/r/sx7aAB/2/list-substitution).

> [!TIP]
> When you copy a pattern into the rule, double the backslashes: write `\\d` for
> a digit and `\\s` for a space. As the rule is a string in a YAML file, a single
> backslash does not reach the regular expression engine.

Regular expressions are easy to get wrong. An LLM can write the whole block for
you. Give it the reference documentation and your own descriptions. In the
example below, they do not follow the convention of the shipped rules: the
provider comes first and the type of connection is in brackets.

```
Read this page about the classification language of Akvorado:
https://demo.akvorado.net/api/v0/console/docs/configuration

Here are the descriptions of the interfaces of my router:

Cogent [TRANSIT] AS174 / 1-3834938493
Netflix [PNI] AS2906 / WL6-1190
FranceIX [IX] AS51706
dc3-edge1 [BACKBONE]
dc3-edge2 [BACKBONE]

Write the interface-classifiers block for config/outlet.yaml. External
interfaces get their connectivity type and their provider, everything else is
internal. Answer with the YAML block only to paste in my configuration.
```

Paste the answer under `core` in `config/outlet.yaml`. You can check the regular
expressions it wrote with the two links above.

After applying a change, you need to restart the outlet:

```console
# docker compose restart outlet
```

## Send flows from your router

Configure your router to sample its traffic and to send it to the machine that
runs *Akvorado*. The ports are:

- 2055 for NetFlow,
- 4739 for IPFIX,
- 6343 for sFlow.

The exact commands depend on the platform. Take the snippet for yours in the
[exporters guide](11-exporters.md) and adapt the addresses. Sample on ingress,
on every interface, and use active and inactive timeouts of 10 seconds.

If you have no router at hand, a Linux machine can also export flows with
[pmacct or ipfixprobe](11-exporters.md#gnulinux).

## Check that the flows arrive

> [!NOTE]
> Run the `curl` commands below on the server itself. Port 8080 exposes the
> metrics and the configuration without authentication, so it is only published
> on the loopback interface. Only port 8081, the console, is reachable from
> outside.

The inlet counts the packets it receives, per exporter:

```console
$ curl -s http://127.0.0.1:8080/api/v0/inlet/metrics | grep 'akvorado_inlet_flow_input_udp_packets'
akvorado_inlet_flow_input_udp_packets_total{exporter="203.0.113.1",listener=":2055",worker="0"} 128
```

If your router is not in the list, it does not reach the inlet. If it is, ask
the outlet for one processed flow:

```console
$ curl -s http://127.0.0.1:8080/api/v0/outlet/flows\?limit\=1
{"TimeReceived":1753631373,"SamplingRate":1000,"ExporterAddress":"::ffff:203.0.113.1","InIf":10,[...]}
```

A flow with an `InIfName` and an `InIfBoundary` means the whole chain works: the
router exports, the inlet receives, the outlet enriches. When something is
missing, the [troubleshooting guide](12-troubleshooting.md) goes through each
component in order.

## Look at your traffic

Wait a few minutes for the data to pile up, then open
`http://127.0.0.1:8081` again. The home page counts your flows and lists your
exporter. In the “Visualize” tab, the default graph shows your top source AS
numbers.

You now have a working collector. From here:

- add your other routers, one at a time,
- give *Akvorado* your BGP routes over BMP to get AS paths and communities, as
  described in the [configuration reference](50-configuration.md#routing),
- read the [operating guide](13-operating.md) before you rely on this setup.
