# Installation

*Akvorado* is written in Go. It provides its 3 components into a
single binary or Docker image.

## Compilation from source

You need a proper installation of [Go](https://go.dev/doc/install)
(1.17+) as well as [NodeJS](https://nodejs.org/en/download/). Then,
simply type:

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

## Quick start

A `docker-compose.yml` file is also provided to quickly get started.
Once running, *Akvorado* web interface should be running on port 80
and an inlet accepting NetFlow available on port 2055.

```console
# docker-compose up
```

A few synthetic flows are generated in the background. Take a look at
the `docker-compose.yml` file if you want to setup the GeoIP database.
It requires two environment variables to fetch them from
[MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data).

Be sure to flush the conntrack table after starting. See the
[troubleshooting section](05-troubleshooting.md#no-packets-received)
for more details.
