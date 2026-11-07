# BSD ports of Akvorado

Ports of Akvorado for three BSD packaging systems, all pinned to the same
upstream source and auxiliary distfile:

| System  | Location                         | Supported architectures        |
|---------|----------------------------------|--------------------------------|
| FreeBSD | `freebsd/ports/net-mgmt/akvorado` | amd64, aarch64, i386, armv7    |
| pkgsrc  | `pkgsrc/net/akvorado`             | NetBSD/amd64 (others untested) |
| OpenBSD | `openbsd/ports/net/akvorado`      | amd64, aarch64                 |

The architecture limits come from `modernc.org/libc`, pulled by the SQLite
driver used by the console.

- Version: `2026.10.1.20261104`
- Source: [akvorado/akvorado](https://github.com/akvorado/akvorado) at
  commit `472d8382d88e61f5181c13880b02e2808480075c` (upstream `main`
  after the `v2026.10.1` release).

## Generated files distfile

Akvorado embeds files that upstream produces with `make all-indep` and does
not commit: the built web console (`common/embed/data/embed.zip`), generated
Go code (protobuf, parser, enumer) and ClickHouse data (`asns.csv`, …).
Generating them needs Node.js, pnpm and network access, which a port build
cannot have. The ports therefore fetch them as an auxiliary distfile:

```
akvorado-2026.10.1.20261104-generated-assets.tar.gz
SHA256 0f3be371d44c0c4aff70f9c6c688962ed15301ad12fc98b3fb26ae63e3f09c3c
```

The archive uses an `akvorado-<commit>/` prefix, matching the directory of
the GitHub source archive, so every framework extracts it straight into
`WRKSRC` with no custom extract step. It is published as a release asset of
this repository (tag `bsd-distfiles-<version>`).

To produce it for a new version, from a checkout of the commit to package:

```sh
touch inlet/flow/input/udp/reuseport_*.o   # use the committed eBPF programs
make all-indep
AKVORADO_PORT_VERSION=<version> bsd/make-generated-assets.sh /tmp
gh release create bsd-distfiles-<version> --target <commit> --prerelease \
    --latest=false /tmp/akvorado-<version>-generated-assets.tar.gz
```

The script is deterministic (fixed owner, mode, order, mtime and gzip
header): the same tree always gives the same checksum.

## Runtime adaptations

The upstream sample configuration targets Docker Compose. All three ports
patch it the same way so that every service can run on a single host:

- Kafka, ClickHouse and orchestrator addresses point to `127.0.0.1`.
- Each service gets its own HTTP port: the default is `:8080` for all of
  them, so they would conflict on one host. The orchestrator uses 8080, the
  inlet 8081, the outlet 8082 and the console 8083.
- The console uses the memory cache instead of Redis.
- The inlet uses one UDP worker per port. On the BSDs, `SO_REUSEPORT` lets
  several sockets bind the same port but does not spread datagrams across
  them, so extra workers would stay idle.
- GeoIP databases are read from the location used by each system.

The orchestrator exits when Kafka is not reachable, and the other services
exit when the orchestrator is not reachable. The FreeBSD rc.d scripts
restart them through `daemon(8)`. The pkgsrc and OpenBSD packages document
the start order.

## Testing

`bsd/smoke-test.sh` builds both binaries like the ports do and runs the
tests of the platform-specific network inputs. It then starts the four
services on one host with the adapted configuration and an in-memory Kafka
broker (`bsd/kfake.go`), and checks each healthcheck. CI runs it on FreeBSD,
NetBSD and OpenBSD VMs (`build-bsd` job). To run it by hand on a BSD host:

```sh
# extract the source archive, then the generated-assets distfile over it
cd akvorado-<commit> && sh bsd/smoke-test.sh
```

Then validate each port with its own tools: `make makesum`, `make stage`,
`make check-plist`, `portlint -AC` and `poudriere testport` on FreeBSD;
`pkglint` and `make package` on pkgsrc; `make makesum`, `make update-plist`,
`make port-lib-depends-check` and `portcheck` on OpenBSD.

## Before submitting

- FreeBSD: apply `freebsd/ports/UIDs-GIDs.diff` to the ports tree (UID/GID
  830 was free in `UIDs`/`GIDs` on 2026-11-06).
- OpenBSD: UID/GID 906 was the next free ID in
  `infrastructure/db/user.list` on 2026-11-06. Add the `_akvorado` line there.
- Move the generated-assets distfile to a location the ports projects accept
  for long-term hosting (maintainer distfile area, or an upstream release
  asset if upstream agrees to publish one).
