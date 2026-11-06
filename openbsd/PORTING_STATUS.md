# OpenBSD Port Work Status

Updated: 2026-11-06

## Current Environment

- Host: FreeBSD 15.1-STABLE amd64.
- Target VM: `OpenBSD-NOGGlass`.
- Guest verification: `OpenBSD 8.0 GENERIC.MP#84 amd64`.
- Downloaded OpenBSD 8.0 `ports.tar.gz`; `signify -C` verified it against the
  signed SHA256 manifest using `/etc/signify/openbsd-80-base.pub`. Extracted it
  to `/usr/ports` after confirming the path was absent.
- `doas -n` works in the guest. `/usr/ports/mystuff/net/akvorado` is ready for
  local port testing.
- The draft has not been copied into that directory: the guest's HTTP request
  to the host stalled, and non-interactive `scp` authentication was unavailable.
- Go 1.27.1 is installed. From the extracted pinned source archive,
  `GOTOOLCHAIN=local go list -m` succeeds and prints `akvorado`.
- The source archive SHA256 matches the value in
  `freebsd/ports/net-mgmt/akvorado/distinfo`.
- The host's `/usr/ports` remains a FreeBSD tree and must not be used for
  OpenBSD port builds.

## Phase 3 Files

- `ports/net/akvorado/Makefile` pins the GitHub source commit and declares the
  Go framework and basic package metadata.
- `ports/net/akvorado/pkg/DESCR` describes the application and external
  Kafka/ClickHouse requirements.
- These are an initial, unvalidated skeleton, not a buildable OpenBSD port.
- `MODGO_GO111MODULE=off` is explicit in the draft: without `MODGO_MODNAME`,
  OpenBSD intentionally configures an invalid `GOPROXY`.
- `modules.inc`, `distinfo`, and `pkg/PLIST` are intentionally absent. They
  must be generated or verified using OpenBSD's own ports tools after the Go
  source/module integration is resolved.

## Blocking Items

1. Transfer the local skeleton into `/usr/ports/mystuff/net/akvorado` and
  validate its effective OpenBSD port variables.
2. Resolve source and dependency distfiles together: `MODGO_MODNAME` enables
  module mode and the local proxy but also adds the main module archive from
  `proxy.golang.org`. The bare module name `akvorado` is not the GitHub source
  URL.
3. Generate complete, checksummed module distfiles and an offline local GOPROXY
  using OpenBSD 8.0 ports tools. The helper uses `proxy.golang.org` and
  `pkg.go.dev`; a lookup and direct request did not return during this session,
  so module generation remains blocked/unverified.
4. Provide `common/embed/data/embed.zip` reproducibly, then run `make fetch`,
  `make checksum`, and `make build` in the OpenBSD ports framework.
5. Generate and review `pkg/PLIST` from `make fake`/`make update-plist`.

Do not treat the Go builds previously run on FreeBSD as evidence that the
OpenBSD port works.
