# Installation

*Akvorado* is written in Go. It provides its 3 components into a
single binary or Docker image.

## Compilation from source

You need a proper installation of [Go](https://go.dev/doc/install)
(1.17+) as well as
[Yarn](https://yarnpkg.com/getting-started/install). Then, simply
type:

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

It is also possible to build a Docker image without installing
anything else than [Docker](https://docs.docker.com/get-docker):

```console
# docker build . -t akvorado:main
```

A `docker-compose.yml` file is also provided to quickly get started.
Once running, *Akvorado* web interface should be running on port 80
and an inlet accepting NetFlow available on port 2055.

```console
# env GEOIPUPDATE_ACCOUNT_ID=xxxx GEOIPUPDATE_LICENSE_KEY=xxxx  docker-compose up
```

The two environment variables are required to get GeoIP database from
[MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).
