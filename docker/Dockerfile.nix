FROM nixpkgs/nix-flakes:latest AS build
ARG TARGETPLATFORM
RUN echo filter-syscalls = false >> /etc/nix/nix.conf
WORKDIR /app
COPY . .
RUN mkdir -p /output/store
RUN make version > .version && git add -f .version
RUN nix run ".#update" \
 && nix run ".#build" \
 && cp -va $(nix-store -qR result) /output/store \
 && rm -rf /output/store/*-akvorado

FROM gcr.io/distroless/static:latest
LABEL org.opencontainers.image.title="Akvorado"
LABEL org.opencontainers.image.description="Flow collector, enricher and visualizer (nix)"
LABEL org.opencontainers.image.source=https://github.com/akvorado/akvorado
LABEL org.opencontainers.image.licenses=AGPL-3.0-only
COPY --from=build /output/store /nix/store
COPY --from=build /app/result/  /usr/local/
EXPOSE 8080
HEALTHCHECK --interval=20s CMD [ "/usr/local/bin/akvorado", "healthcheck" ]
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
