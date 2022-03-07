# Flow exporter

This program receives flows (currently Netflow), enriches them with
interface names (using gNMI), geo information (using MaxMind) and
export them to Kafka.

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

Check `bin/flowexporter server --help` for help to run the exporter.
Dump the default configuration with `bin/flowexporter serve --check
--dump`.

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
