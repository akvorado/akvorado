# Installation

*Akvorado* is written in Go. It provides its 4 components in a single binary or
Docker image. It also requires [Kafka](https://kafka.apache.org/quickstart) and
[ClickHouse](https://clickhouse.com/docs/en/getting-started/install/), which
must be installed separately. The minimum version for ClickHouse is 22.4.

## Docker image

You can use the *Akvorado* [Docker](https://docs.docker.com/get-docker) image.

```console
# docker pull ghcr.io/akvorado/akvorado:latest
# docker run --rm ghcr.io/akvorado/akvorado:latest help
```

Check the `docker/docker-compose.yml` file for an example of how to deploy
*Akvorado* using containers. If you want to use `docker compose`, see
the [quick start procedure](00-intro.md#quick-start). This documentation assumes
you are using the `docker compose` setup.

If you want to compile the Docker image yourself, use `make docker`.

## Pre-built binary

The second option is to download a pre-built binary from the [release page
on GitHub](https://github.com/akvorado/akvorado/releases).
Currently, only a pre-built binary for Linux x86-64 is provided.

## Compilation from source

You need to install [Go](https://go.dev/doc/install) (1.21+), and
[NodeJS](https://nodejs.org/en/download/) (20+) with NPM (6+). For example, on
Debian:

```console
# apt install golang nodejs npm
# go version
go version go1.24.1 linux/amd64
# node --version
v20.19.2
# npm --version
9.2.0
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

Read the [changelog](99-changelog.md) before you upgrade.
Upgrade the orchestrator first. This will update the ClickHouse database if
needed. Then, upgrade all inlets and outlets. Then the console.

When using `docker compose`, use the following commands to get an updated
`docker-compose.yml` file and update your installation.

```console
# cd akvorado
# curl -sL https://github.com/akvorado/akvorado/releases/latest/download/docker-compose-upgrade.tar.gz | tar zxvf -
# docker compose pull
# docker compose stop akvorado-orchestrator
# docker compose up -d
```

The `docker-compose-upgrade.tar.gz` tarball contains `.env.dist` instead of
`.env`, and `docker/docker-compose-local.yml.dist` instead of
`docker/docker-compose-local.yml.dist`. You may want to check for differences
with your setup.
