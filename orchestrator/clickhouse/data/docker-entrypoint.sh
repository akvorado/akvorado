#!/bin/sh

set -ex

rm -f /tmp/init.sh
while [ ! -s /tmp/init.sh ]; do
    sleep 1
    wget --no-proxy -qO /tmp/init.sh http://akvorado-orchestrator:8080/api/v0/orchestrator/clickhouse/init.sh || continue
done
sh /tmp/init.sh
