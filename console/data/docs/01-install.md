# Installation

*Akvorado* is written in Go. It provides its 3 components into a
single binary or Docker image. It also requires an installation of
[Kafka](https://kafka.apache.org/quickstart) and
[ClickHouse](https://clickhouse.com/docs/en/getting-started/install/).
They have to be installed separately.

## Compilation from source

You need a proper installation of [Go](https://go.dev/doc/install)
(1.18+), [NodeJS](https://nodejs.org/en/download/), and
[protoc](https://grpc.io/docs/protoc-installation/). For example, on
Debian:

```console
# apt install golang-1.18 nodejs npm protobuf-compiler
```

Then, simply type:

```console
# make
▶ running gofmt…
▶ running golint…
▶ building executable…
```

The resulting executable is `bin/akvorado`.

The following `make` targets are available:

 - `make help` to get help
 - `make` to build the binary (in `bin/`)
 - `make test` to run tests
 - `make test-verbose` to run tests in verbose mode
 - `make test-race` for race tests
 - `make test-xml` for tests with xUnit-compatible output
 - `make test-coverage` for test coverage (will output `index.html`,
   `coverage.xml` and `profile.out` in `test/coverage.*/`.
 - `make test PKG=helloworld/hello` to restrict test to a package
 - `make clean`
 - `make lint` to run golint
 - `make fmt` to run gofmt

## Docker image

It is also possible to get Akvorado as a
[Docker](https://docs.docker.com/get-docker) image:

```console
# docker pull ghcr.io/vincentbernat/akvorado:latest
```
