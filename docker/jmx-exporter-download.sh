#!/bin/sh

set -e

VERSION=1.3.0
NAME=jmx_prometheus_javaagent-${VERSION}.jar
URL=https://github.com/prometheus/jmx_exporter/releases/download/${VERSION}/${NAME}

cd /opt/jmx-exporter

# Check if target version already exist
[ ! -s jmx_prometheus_javaagent-${VERSION}.jar ] || exit 0

# Retrieve it
apk add --no-cache curl
curl --retry 10 --retry-connrefused --remove-on-error --remote-name --fail --silent --location $URL
ln -vsf ${NAME} jmx_prometheus_javaagent.jar
