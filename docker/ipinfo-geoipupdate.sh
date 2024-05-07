#!/bin/sh

trap exit TERM

while true; do
    for DATABASE in ${IPINFO_DATABASES}; do
        RESPONSE=$(curl \
            -s -w '%{http_code}' -L -o "${DATABASE}.mmdb.new" \
            "https://ipinfo.io/data/free/${DATABASE}.mmdb?token=${IPINFO_TOKEN}")
        if [ "$RESPONSE" != "200" ]; then
            echo "$RESPONSE Failed to download ${DATABASE}.mmdb database."
            rm "${DATABASE}.mmdb.new" 2> /dev/null
        else
            echo "${DATABASE}.mmdb database downloaded in /data volume."
            mv "${DATABASE}.mmdb.new" "${DATABASE}.mmdb"
        fi
    done

    sleep "$UPDATE_FREQUENCY" &
    wait $!
done
