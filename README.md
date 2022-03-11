# Akvorado: flow collector, enricher and exporter.

This program receives flows (currently Netflow), enriches them with
interface names (using SNMP), geo information (using MaxMind), and
exports them to Kafka.

[Akvorado][] means "water wheel" in Esperanto.

[Akvorado]: https://eo.wikipedia.org/wiki/Akvorado

## Build

Use `make`. The following commands are available:

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

## Run

Check `bin/akvorado server --help` for help to run the exporter.
Dump the default configuration with `bin/akvorado serve --check
--dump`.

The embedded HTTP server contains the following endpoints:

 - `/metrics` for Prometheus metrics,
 - `/version` for the running version,
 - `/healthcheck` telling if we are alive,
 - `/flow.proto` for the Protobuf schema for flows exported to Kafka,

## Design

### Components

The generic design is component-based. Each piece is a component. You
get an instance of the component with `New()` (or `NewMock()` for
tests). Each component is initialized with:
- an instance of the reporter (the component for logging),
- their configuration (extracted from the general configuration),
- the components they depend on.

Each component maintain its state. They may have a `Start()` and a
`Stop()` method.

See https://github.com/stuartsierra/component to understand where the
inspiration comes from.
