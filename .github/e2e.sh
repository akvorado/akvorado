#!/bin/sh

# Helper script for E2E testing. This is meant to be run inside GitHub CI.

set -e

case $1 in
    compose-setup)
        # Setup docker compose for E2E testing.
        # Disable geoip service (rate-limited token needed)
        cat >> docker/docker-compose-local.yml <<EOF
services:
  geoip:
    profiles: [ disabled ]
EOF
        # For each service, collect coverage files
        [ -n "$AKVORADO_COVERAGE_DIRECTORY" ]
        mkdir -p ${AKVORADO_COVERAGE_DIRECTORY}
        for service in orchestrator inlet outlet console exporter-1 conntrack-fixer; do
            cat >> docker/docker-compose-local.yml <<EOF
  akvorado-${service}:
    volumes:
      - ${AKVORADO_COVERAGE_DIRECTORY}/${service}:/tmp/coverage
    environment:
      GOCOVERDIR: /tmp/coverage
EOF
        done
        ;;

    tests)
        echo ::group::Get Hurl
        nix run nixpkgs#hurl -- --version
        echo ::endgroup::

        echo ::group::Wait for first flow
        while true; do
            sleep 1
            ! curl -o /dev/null --write-out "%{url}: %{response_code}\n" -sf \
                http://127.0.0.1:8080/api/v0/console/widget/flow-last || break
        done
        echo ::endgroup::

        echo ::group::Wait for Prometheus to be ready
        promtool() {
            docker compose exec prometheus promtool "$@"
        }
        promtool_query() {
            promtool query instant http://localhost:9090/prometheus "$@"
        }
        promtool check healthy --url=http://localhost:9090/prometheus
        while true; do
            promtool_query up
            dcount=$(docker container ps --filter "label=metrics.port" --format "{{.Names}}" | wc -l)
            pcount=$(promtool_query up | wc -l)
            # We have two non-Docker sources: Kafka and Redis
            [ $pcount -ne $((dcount + 2)) ] || break
            sleep 1
        done
        promtool_query 'akvorado_cmd_info{job=~"akvorado-.+"}'
        promtool query labels http://localhost:9090/prometheus job
        echo ::endgroup::

        # Check Loki status
        # This is difficult to include tests for Loki as Vector do not read logs
        # before it started. See https://github.com/vectordotdev/vector/issues/7358

        # echo ::group::Check Loki status
        # export LOKI_ADDR=http://localhost:8080/loki
        # logcli() {
        #     nix shell nixpkgs#grafana-loki --command logcli "$@"
        # }
        # logcli -q labels service_name
        # logcli -q series '{service_name=~".+"}' --analyze-labels
        # echo ::endgroup::

        # Run Hurl tests. Some of them may be difficult to get right, notably
        # the ClickHouse batches (if we are rebalancing). We need to let some
        # time between two tries because the quantiles need to be updated.
        echo ::group::Hurl tests
        ok=0
        i=1
        t=2
        while [ $t -lt 100 ]; do
            echo "::group::Hurl tests ($i)"
            if ! nix run nixpkgs#hurl -- --test --error-format=short .github/e2e.hurl; then
                echo ::endgroup::
                sleep $t
                i=$((i+1))
                t=$((t*2))
                continue
            else
                echo ::endgroup::
                ok=1
                break
            fi
        done
        [ $ok -eq 1 ] || {
            sleep 10
            nix run nixpkgs#hurl -- --test --error-format=long .github/e2e.hurl
        }
        echo ::endgroup::
        ;;

    coverage)
        # Merge coverage files
        [ -n "$AKVORADO_COVERAGE_DIRECTORY" ]
        mkdir -p ${AKVORADO_COVERAGE_DIRECTORY}/all
        inputs=$(cd ${AKVORADO_COVERAGE_DIRECTORY} ; ls | xargs readlink -f | grep -v /all$ | paste -sd ,)
        go tool covdata merge \
            -i=${inputs} \
            -o=${AKVORADO_COVERAGE_DIRECTORY}/all
        go tool covdata textfmt \
            -i=${AKVORADO_COVERAGE_DIRECTORY}/all \
            -o=${AKVORADO_COVERAGE_DIRECTORY}/e2e-coverage.out
        ;;

esac
