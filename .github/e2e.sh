#!/bin/sh

# Helper script for E2E testing. This is meant to be run inside GitHub CI.

set -e

[ -n $AKVORADO_COVERAGE_DIRECTORY ]

mkdir -p ${AKVORADO_COVERAGE_DIRECTORY}

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
        # Validate the first incoming flow
        while true; do
            sleep 1
            ! curl -o last-flow.json --write-out "%{url}: %{response_code}\n" -sf \
                http://127.0.0.1:8080/api/v0/console/widget/flow-last || break
        done
        < last-flow.json \
            jq -e '(.InIfName | test("^Gi0/.*")) and (.OutIfName | test("^Gi0/.*")) and .ExporterRole == "edge"'
        # Validate the various metrics endpoints
        for component in inlet outlet console orchestrator; do
            curl -o /dev/null --write-out "%{url}: %{response_code}\n" -sf \
                http://127.0.0.1:8080/api/v0/${component}/metrics || break
        done
        ;;

    coverage)
        # Merge coverage files
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
