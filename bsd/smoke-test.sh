#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-only
#
# Smoke test for the BSD ports. Run from the top of a source tree that
# contains the generated files (after "make all-indep", or after extracting
# the generated-assets distfile), with Go in PATH:
#
#   bsd/smoke-test.sh
#
# It builds both binaries like the ports do, runs the tests of the
# platform-specific network inputs, then starts the orchestrator, inlet,
# outlet and console together on this host, using the sample configuration
# adapted exactly like the ports adapt it, and an in-memory Kafka broker
# (bsd/kfake.go). ClickHouse is not needed: the orchestrator only logs errors
# about it. Each service must answer its healthcheck on its own HTTP port.
#
# Set AKVORADO_BIN to a directory holding prebuilt akvorado and
# akvorado-inlet binaries to test an installed package instead.

set -eu

export CGO_ENABLED=0 GOTOOLCHAIN=local GOTELEMETRY=off
tags=release,grpcnotrace,validator_novalidatefn
work=$(mktemp -d "${TMPDIR:-/tmp}/akvorado-smoke.XXXXXX")
pids=
cleanup() {
    status=$?
    for pid in $pids; do kill "$pid" 2> /dev/null || true; done
    wait 2> /dev/null || true
    if [ $status -ne 0 ]; then
        for log in "$work"/*.log; do
            [ -f "$log" ] || continue
            echo "--- $(basename "$log")" >&2
            tail -20 "$log" >&2
        done
    fi
    rm -rf "$work"
    exit $status
}
trap cleanup EXIT INT TERM

step() { printf '\n▶ %s\n' "$*"; }

step "System: $(uname -srm)"
go version

if [ -n "${AKVORADO_BIN:-}" ]; then
    bin=$AKVORADO_BIN
else
    bin=$work/bin
    step "Building akvorado and akvorado-inlet"
    go build -trimpath -tags=$tags -o "$bin/akvorado" ./cmd/akvorado
    go build -trimpath -tags=$tags,akvorado_inlet -o "$bin/akvorado-inlet" ./cmd/akvorado
fi
"$bin/akvorado" version
"$bin/akvorado-inlet" version

if [ -z "${AKVORADO_BIN:-}" ]; then
    step "Testing platform-specific network inputs"
    go test -count=1 ./inlet/flow/input/udp ./inlet/flow/input/sctp
fi

step "Adapting the sample configuration like the ports do"
cp config/akvorado.yaml config/inlet.yaml config/outlet.yaml config/console.yaml "$work/"
sed -e 's|- kafka:9092|- 127.0.0.1:9092|' \
    -e 's|- clickhouse:9000|- 127.0.0.1:9000|' \
    -e 's|http://akvorado-orchestrator:8080|http://127.0.0.1:8080|' \
    config/akvorado.yaml > "$work/akvorado.yaml"
sed -e 's|workers: 4|workers: 1|' config/inlet.yaml > "$work/inlet.yaml"
sed -e 's|/usr/share/GeoIP|/usr/local/share/GeoIP|' config/outlet.yaml > "$work/outlet.yaml"
sed -e 's|^  cache:$|  listen: :8083|' \
    -e '/^    type: redis$/d' \
    -e '/^    server: redis:6379$/d' \
    config/console.yaml > "$work/console.yaml"
printf 'http:\n  listen: :8081\n' >> "$work/inlet.yaml"
printf 'http:\n  listen: :8082\n' >> "$work/outlet.yaml"
"$bin/akvorado" orchestrator "$work/akvorado.yaml" --check

# Wait until a service answers its healthcheck, or fail after ~60 seconds.
wait_healthy() {
    name=$1 port=$2 pid=$3 i=0
    while ! "$bin/akvorado" healthcheck --http "127.0.0.1:$port" > /dev/null 2>&1; do
        kill -0 "$pid" 2> /dev/null || { echo "✗ $name exited" >&2; return 1; }
        i=$((i + 1))
        [ $i -lt 60 ] || { echo "✗ $name not healthy on :$port" >&2; return 1; }
        sleep 1
    done
    echo "✓ $name healthy on :$port"
}

step "Starting an in-memory Kafka broker"
go build -o "$work/kfake" bsd/kfake.go
"$work/kfake" 9092 > "$work/kfake.log" 2>&1 &
pids="$pids $!"
sleep 2

step "Starting the four services on a single host"
(cd "$work" && exec "$bin/akvorado" orchestrator akvorado.yaml) > "$work/orchestrator.log" 2>&1 &
orchestrator=$!
pids="$pids $orchestrator"
wait_healthy orchestrator 8080 $orchestrator

"$bin/akvorado-inlet" inlet http://127.0.0.1:8080 > "$work/inlet.log" 2>&1 &
inlet=$!
"$bin/akvorado" outlet http://127.0.0.1:8080 > "$work/outlet.log" 2>&1 &
outlet=$!
"$bin/akvorado" console http://127.0.0.1:8080 > "$work/console.log" 2>&1 &
console=$!
pids="$pids $inlet $outlet $console"
wait_healthy inlet 8081 $inlet
wait_healthy outlet 8082 $outlet
wait_healthy console 8083 $console

step "Smoke test passed"
