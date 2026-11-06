# OpenBSD Port Roadmap

Status as of 2026-11-06. This is a working plan, not a finished port.

## Progress Through Phase 3

### Phase 1: OpenBSD Environment

- The selected target is `OpenBSD-NOGGlass`; it is reachable over SSH and
   reports `OpenBSD 8.0 GENERIC.MP#84 amd64`.
- The official OpenBSD 8.0 `ports.tar.gz` was downloaded, verified with
   `openbsd-80-base.pub` and `SHA256.sig`, then extracted to `/usr/ports`.
- The host remains FreeBSD 15.1-STABLE; OpenBSD port commands must run in the
   guest, not against the host's `/usr/ports`.
- `doas -n` works in the guest. Go 1.27.1 is installed, and
   `/usr/ports/mystuff/net/akvorado` has been created for local testing.
- The pinned GitHub source archive was downloaded in the guest. Its SHA256
   matches the existing FreeBSD `distinfo`; `GOTOOLCHAIN=local go list -m`
   succeeds from the extracted source tree.

### Phase 2: Go Framework Integration

- In OpenBSD 8.0's `go.port.mk`, setting `MODGO_MODNAME` enables module mode and
   a local `GOPROXY=file://${WRKDIR}/go_modules`, populated from `MODGO_MODULES`
   and `MODGO_MODFILES` in `modules.inc`.
- The framework also adds a main-module archive from the same Go proxy. Since
   Akvorado's module path is the bare name `akvorado` and its source is on
   GitHub, the source `DISTFILES`/`SITES` must be overridden without disrupting
   the dependency distfiles or `WRKSRC` setup.
- With `MODGO_MODNAME` unset, module mode defaults off and `GOPROXY` is
   deliberately invalid, so merely setting `MODGO_GO111MODULE=on` is not enough.
- `modgo-gen-modules-helper` uses `proxy.golang.org` and `pkg.go.dev`; its
   lookup for `akvorado` and a direct proxy request produced no response during
   this session and had to be interrupted. This does not establish whether the
   module is published, but leaves dependency generation unverified.
- Go 1.27.1 accepts the source's `go.mod` with `GOTOOLCHAIN=local` for
   `go list -m`; a full dependency resolution and build have not been tested.

### Phase 3: Initial Skeleton

- Added `ports/net/akvorado/Makefile` and `ports/net/akvorado/pkg/DESCR`.
- The Makefile is a draft only; it does not yet set `MODGO_MODNAME`, because
   the framework would fetch the main source archive from the Go module proxy.
   `MODGO_GO111MODULE` is explicitly off until module distfiles can be staged.
- The guest `mystuff` directory exists, but the skeleton remains local: guest
   HTTP access to the host stalled, and key-only `scp` was unavailable.
- `modules.inc`, `distinfo`, and `pkg/PLIST` are intentionally absent. Do not
   treat this skeleton as a buildable or tested port. No fetch, checksum, or
   build target has passed yet.
- See [PORTING_STATUS.md](PORTING_STATUS.md) for access constraints and the
   exact remaining blockers.

## Starting Point

- Source branch: `main`, commit `83085971` (`Add NetBSD CI and BSD packages`).
- Package version: `2026.10.1.20261104`.
- Source revision used by the existing BSD packages:
  `472d8382d88e61f5181c13880b02e2808480075c`.
- `go.mod` requires Go 1.27 and toolchain 1.27.3. The OpenBSD 8.0 ports tree
   used for this work advertises Go 1.27.1 in `lang/go`.
- Existing implementation references: `freebsd/ports/net-mgmt/akvorado/`
  and `pkgsrc/net/akvorado/`.
- First target: OpenBSD -current/amd64. Expand architectures only after this
  builds and packages cleanly.

## Plan for the Next Session

1. Continue using the `OpenBSD-NOGGlass` VM. Its release is confirmed as
   OpenBSD 8.0 and the matching ports tree has been extracted. The
   `/usr/ports/mystuff/net/akvorado` working directory is ready; keep testing
   there and do not edit the checked-out ports tree directly. Do not use the
   older `OpenBSD-Port` VM.
2. Resolve the Go ports-framework integration before writing the full port.
   Akvorado declares the bare module path `akvorado`, while its source is
   distributed from GitHub at a fixed commit. Verify how `MODGO_MODNAME`,
   `MODGO_VERSION`, `GH_COMMIT`, and `modules.inc` can be combined without
   making the framework fetch the main source from an invalid Go proxy path.
   Generate dependency metadata with the OpenBSD `modgo-gen-modules` helper
   where applicable; do not copy pkgsrc's generated module list blindly.
3. Build the minimum port skeleton using the current OpenBSD template and
   `lang/go` module. Pin the source revision, record checksums with `make
   makesum`, and ensure all Go module downloads are staged as distfiles. Builds
   must use the framework's offline module cache and `GOTOOLCHAIN=local`.
4. Reproduce the two binaries from the existing recipes, preserving their
   release tags and version ldflags:
   - `akvorado`: `release,grpcnotrace,validator_novalidatefn`
   - `akvorado-inlet`: the same tags plus `akvorado_inlet`
5. Handle generated embedded assets explicitly. `common/embed/data/embed.zip`
   is generated from the frontend, avatars, documentation and ClickHouse CSV
   data, and is not committed in the source tree. Decide whether to generate it
   in the port using available build dependencies or provide a pinned,
   reproducible auxiliary distfile. The FreeBSD port's
   `files/generated-assets.tar.gz` is a reference, not an automatic OpenBSD
   solution.
6. Define package contents and runtime behavior: install both binaries,
   README and sample configuration; verify OpenBSD `@sample` handling under
   `/etc`; document Kafka and ClickHouse as external services. Do not copy the
   FreeBSD rc scripts. Decide whether to provide OpenBSD-native rc.d scripts or
   leave service supervision to the administrator, following OpenBSD policy.
7. Test the non-Linux paths. eBPF/SO_REUSEPORT implementation files and the
   SCTP-specific socket options are Linux-only, but compile and run targeted
   tests on OpenBSD to find unsupported syscalls or dependency assumptions.

## Acceptance Checks

Run the OpenBSD port targets step by step and retain logs:

1. `make fetch`, `make checksum`, `make extract`, `make patch`
2. `make configure`, `make build`, `make test`, `make fake`
3. `make update-plist`, `make port-lib-depends-check`, `make package`
4. Install, smoke-test both binaries (including `version`), uninstall, and
   `make clean`.
5. Run `/usr/ports/infrastructure/bin/portcheck` and verify the checksum and
   PLIST are reproducible. Confirm the build does not access the network.
6. After amd64 passes, assess at least arm64 and any OpenBSD architectures the
   Go ports framework supports.

Before submission, check licensing and dependencies, review the final diff,
and request review/testing on `ports@openbsd.org` per the Porting Guide. Do not
attempt an import until the port is tested and has received the required
ports-developer approval.

## Main Risks to Resolve

- How to fetch the GitHub-pinned source while using OpenBSD's Go module staging
  for the dependency graph with the non-canonical main module path.
- How to create `embed.zip` reproducibly without enabling network access during
  the package build.
- Whether the Go application and its dependencies compile on OpenBSD with
  `CGO_ENABLED=0`, especially the network input and socket paths.
- How to express configuration samples and optional service startup in a way
  that follows OpenBSD packaging policy.

## References

- [OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html)
- [OpenBSD Port Testing Guide](https://www.openbsd.org/faq/ports/testing.html)
- [OpenBSD ports FAQ](https://www.openbsd.org/faq/ports/)
- [Go ports module framework](https://github.com/openbsd/ports/blob/master/lang/go/go.port.mk)
- [OpenBSD Go port](https://github.com/openbsd/ports/blob/master/lang/go/Makefile)