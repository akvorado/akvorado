FROM nixpkgs/nix-flakes:latest AS build
WORKDIR /app
COPY . .
RUN mkdir -p /output/store
RUN git describe --tags --always --dirty --match=v* > .version && git add -f .version
RUN nix run ".#passthru.update" \
 && nix build -L \
 && cp -va $(nix-store -qR result) /output/store \
 && rm -rf /output/store/*-akvorado

FROM scratch
LABEL org.opencontainers.image.source=https://github.com/akvorado/akvorado
LABEL org.opencontainers.image.description="Akvorado: flow collector, enricher and visualizer"
LABEL org.opencontainers.image.licenses=AGPL-3.0-only
COPY --from=build /output/store /nix/store
COPY --from=build /app/result/  /usr/local/
ENV SSL_CERT_FILE=/usr/local/share/ca-certificates/ca-bundle.crt
EXPOSE 8080
HEALTHCHECK CMD [ "/usr/local/bin/akvorado", "healthcheck" ]
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
