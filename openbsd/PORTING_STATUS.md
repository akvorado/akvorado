# OpenBSD Port Work Status

Updated: 2026-11-06

See also [../bsd/README.md](../bsd/README.md) for what the three BSD ports
share: the generated-assets distfile, configuration adaptations and the smoke
test.

## Resolved

- **Main module fetch.** `MODGO_MODNAME=akvorado` enables module mode and
  the local `GOPROXY=file://${WRKDIR}/go_modules`. The framework fetches the
  main module as `${SITES}${MODGO_VERSION}.zip`. With
  `SITES=https://github.com/akvorado/akvorado/archive/` and
  `MODGO_VERSION=<commit>`, that is the GitHub archive of the pinned commit,
  so the Go proxy is never asked for the non-fetchable module `akvorado`.
  `WRKSRC` is set to `akvorado-<commit>`.
- **Dependencies.** `modules.inc` has 171 `MODGO_MODULES` and 117
  `MODGO_MODFILES` entries. They come from `go mod download` in a clean
  module cache, the same set as pkgsrc `go-modules.mk`. An offline build
  with only these modules in a `file://` proxy succeeds for openbsd/amd64
  and openbsd/arm64, both binaries.
- **`embed.zip` and the other generated files** come from the shared
  `generated-assets` distfile (`DISTFILES.assets`), which is extracted
  into `WRKSRC`.
- **Packaging.** Native rc.d scripts use `rc_bg=YES`, because Akvorado runs
  in the foreground. The `_akvorado` user/group is 906, the next free ID
  in `user.list`. Samples are installed under
  `${SYSCONFDIR}/akvorado` with `@sample`, and `pkg/README` documents the
  setup.
- **Architectures.** `ONLY_FOR_ARCHS = amd64 aarch64`. The other
  architectures fail on `modernc.org/libc`.

## Still to do on the OpenBSD VM

`distinfo` was computed outside the ports tree. `pkg/PLIST` was written by
hand. Neither has been through the OpenBSD tools yet.

1. Copy `ports/net/akvorado` to `/usr/ports/mystuff/net/akvorado`.
2. `make makesum`. It should not change `distinfo` once the
   `bsd-distfiles-2026.10.1.20261104` release asset is published.
3. `make build`, `make test`, `make fake`, `make update-plist` (review the
   diff with `pkg/PLIST`), and `make port-lib-depends-check`.
4. `make package`. Install the package and run `sh bsd/smoke-test.sh`
   with `AKVORADO_BIN=/usr/local/bin` from an extracted source tree. Then
   check `rcctl start`/`stop`/`check` for each service.
5. Run `portcheck`, then send the port to ports@openbsd.org for review.

Do not treat Go builds on FreeBSD or macOS as evidence that the OpenBSD
package works.
