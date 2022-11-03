FROM nixpkgs/nix-flakes:latest AS build
WORKDIR /app
COPY flake.nix ./
COPY flake.lock ./
RUN nix develop -c true
# Build
COPY . .
RUN mkdir -p /output/store
RUN git describe --tags --always --dirty --match=v* > .version && git add -f .version
RUN nix build --option sandbox false
RUN cp -va $(nix-store -qR result) /output/store
RUN rm -rf /output/store/*-akvorado

FROM scratch
COPY --from=build /output/store /nix/store
COPY --from=build /app/result/  /usr/local/
ENV SSL_CERT_FILE=/usr/local/share/ca-certificates/ca-bundle.crt
EXPOSE 8080
HEALTHCHECK CMD [ "/usr/local/bin/akvorado", "healthcheck" ]
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
