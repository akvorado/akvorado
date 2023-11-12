#!/bin/bash

set -e

if [[ $# -lt 1 ]] || [[ "$1" = "--"* ]]; then
    rm -f /tmp/init.sh
    while [[ ! -s /tmp/init.sh ]]; do
        sleep 1
        echo "Downloading ClickHouse init script..."
        wget --no-proxy -qO /tmp/init.sh \
            http://akvorado-orchestrator:8080/api/v0/orchestrator/clickhouse/init.sh || continue
    done
    sh /tmp/init.sh
fi

# Use official entrypoint
exec /entrypoint.sh "$@"
