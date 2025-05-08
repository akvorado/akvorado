#!/bin/sh

trap exit TERM

while true; do
    ok=1
    for DATABASE in ${IPINFO_DATABASES}; do
        if [ -f ${DATABASE}.mmdb ]; then
            # Is it up-to-date?
            LOCAL=$(sha256sum ${DATABASE}.mmdb | awk '{print $1}')
            REMOTE=$(curl --silent https://ipinfo.io/data/free/${DATABASE}.mmdb/checksums?token=${IPINFO_TOKEN} \
                | sed -n 's/.*"sha256": *"\([a-f0-9]*\)".*/\1/p')
            if [ "$LOCAL" = "$REMOTE" ]; then
                echo "${DATABASE}.mmdb is up-to-date"
                continue
            fi
        fi
        RESPONSE=$(curl \
            --silent \
            --write-out '%{http_code}' \
            --remote-time \
            --location \
            --output "${DATABASE}.mmdb.new" \
            "https://ipinfo.io/data/free/${DATABASE}.mmdb?token=${IPINFO_TOKEN}")
        case "$RESPONSE" in
            200)
                echo "${DATABASE}.mmdb database downloaded in /data volume."
                mv "${DATABASE}.mmdb.new" "${DATABASE}.mmdb"
                ;;
            *)
                echo "Failed to download ${DATABASE}.mmdb database (HTTP error $RESPONSE)."
                rm "${DATABASE}.mmdb.new" 2> /dev/null
                ok=0
                ;;
        esac
    done

    [ $ok -eq 1 ] && touch /tmp/healthy
    sleep "$UPDATE_FREQUENCY" &
    wait $!
done
