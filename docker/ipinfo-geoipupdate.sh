#!/bin/sh

trap exit TERM

while true; do
    for DATABASE in ${IPINFO_DATABASES}; do
        RESPONSE=$(curl \
            --silent \
            --write-out '%{http_code}' \
            --remote-time \
            --location \
            --output "${DATABASE}.mmdb.new" \
            --time-cond "${DATABASE}.mmdb" \
            "https://ipinfo.io/data/free/${DATABASE}.mmdb?token=${IPINFO_TOKEN}")
        case "$RESPONSE" in
            200)
                echo "${DATABASE}.mmdb database downloaded in /data volume."
                mv "${DATABASE}.mmdb.new" "${DATABASE}.mmdb"
                ;;
            304)
                echo "${DATABASE}.mmdb database not modified."
                ;;
            *)
                echo "Failed to download ${DATABASE}.mmdb database (HTTP error $RESPONSE)."
                rm "${DATABASE}.mmdb.new" 2> /dev/null
                ;;
        esac
    done

    sleep "$UPDATE_FREQUENCY" &
    wait $!
done
