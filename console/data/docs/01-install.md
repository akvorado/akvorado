# Installation

*Akvorado* is written in Go. It provides its 3 components into a
single binary or Docker image. It also requires an installation of
[Kafka](https://kafka.apache.org/quickstart) and
[ClickHouse](https://clickhouse.com/docs/en/getting-started/install/).
They have to be installed separately. For ClickHouse, the minimal
version is 22.4 (due to the use of the `INTERPOLATE` directive).

## Docker image

You can get *Akvorado* as a
[Docker](https://docs.docker.com/get-docker) image.

```console
# docker pull ghcr.io/akvorado/akvorado:latest
# docker run --rm ghcr.io/akvorado/akvorado:latest help
```

Check the `docker-compose.yml` file for an example on how to deploy *Akvorado*
using containers. If you want to use `docker-compose`, have a look at the [quick
start procedure](00-intro.md#quick-start). This documentation assumes you are
running the `docker-compose` setup.

If you want to compile the Docker image yourself, you can use `docker build -t
akvorado:latest .`. Then, in `docker-compose.yml`, replace
`ghcr.io/akvorado/akvorado:latest` by `akvorado:latest`.

## Pre-built binary

The second option is to get a pre-built binary from the [release page
on GitHub](https://github.com/akvorado/akvorado/releases).
Currently, only a pre-built binary for Linux x86-64 is provided.

## Compilation from source

You need a proper installation of [Go](https://go.dev/doc/install) (1.19+), and
[NodeJS](https://nodejs.org/en/download/) (14+) with NPM (6+). For example, on
Debian:

```console
# apt install golang-1.19 nodejs npm
# node --version
v16.15.1
# npm --version
8.14.0
```

Then, type:

```console
# make
▶ building golang.org/x/tools/cmd/goimports@latest…
▶ fetching node modules…
▶ formatting code…
▶ building github.com/mgechev/revive@latest…
▶ running lint…
▶ building google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.0…
▶ compiling protocol buffers definition…
▶ building github.com/golang/mock/mockgen@v1.6.0…
▶ generate mocks for ClickHouse driver…
▶ generate ASN map…
▶ building github.com/mna/pigeon@v1.1.0…
▶ generate PEG parser for filters…
▶ generate list of selectable fields…
▶ building console frontend…
vite v3.0.0 building for production...
✓ 2384 modules transformed.
../data/frontend/assets/akvorado.399701ee.svg   93.44 KiB
../data/frontend/index.html                     0.54 KiB
../data/frontend/assets/index.26bdc6d7.css      68.11 KiB / gzip: 9.84 KiB
../data/frontend/assets/index.64c3c8d1.js       1273.41 KiB / gzip: 429.70 KiB
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
- `make lint` to lint source code
- `make fmt` to format source code

## Upgrade

Be sure to read the [changelog](99-changelog.md) before attempting an upgrade.
Upgrade the orchestrator first. This will update the ClickHouse database if
needed. Then, upgrade all inlets. Then the console.

When using `docker-compose`, use the following commands to fetch an updated
`docker-compose.yml` file and update your installation.

```console
# cd akvorado
# curl -sL https://github.com/akvorado/akvorado/releases/latest/download/docker-compose-quickstart.tar.gz | tar zxvf - docker-compose.yml
# docker-compose pull
# docker-compose stop akvorado-orchestrator
# docker-compose up -d
```

Note that if Zookeeper or Kakfa gets upgraded in the process, this can be
disruptive. Feel free to only use `docker-compose pull akvorado-orchestrator` to
only update Akvorado image.
