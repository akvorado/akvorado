FROM nixpkgs/nix-flakes:latest AS build
WORKDIR /app
COPY flake.nix ./
COPY flake.lock ./
RUN nix develop -c true && nix run .\#curl -- --version
# Build
COPY . .
RUN mkdir -p /output/store
RUN nix build --option sandbox false
RUN cp -va $(nix-store -qR result) /output/store

FROM scratch
COPY --from=build /output/store /nix/store
COPY --from=build /app/result/  /usr/local/
EXPOSE 8080
HEALTHCHECK CMD curl -sf --noproxy localhost:8080 http://localhost:8080/api/v0/healthcheck || exit 1
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
