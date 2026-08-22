ARG NODE_VERSION
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS build-js
RUN apk add --no-cache make
WORKDIR /build
RUN --mount=source=console/frontend,target=console/frontend,rw \
    --mount=source=bin/external-tool,target=bin/external-tool \
    --mount=source=Makefile,target=Makefile \
    make console/data/frontend

FROM --platform=$BUILDPLATFORM golang:alpine AS build-go
RUN apk add --no-cache make curl zip
WORKDIR /build
# Cache for modules
COPY go.mod go.sum .
RUN go mod download
# Build
COPY . .
COPY --from=build-js /build/console/data/frontend console/data/frontend
RUN touch console/frontend/node_modules .fmt-js~ .fmt.go~ .lint-js~ .lint-go~ \
 && find . -print0 | xargs -0 touch -d @0
RUN make all-indep
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION
RUN make

FROM gcr.io/distroless/static:latest
LABEL org.opencontainers.image.title="Akvorado"
LABEL org.opencontainers.image.description="Flow collector, enricher and visualizer"
LABEL org.opencontainers.image.source=https://github.com/akvorado/akvorado
LABEL org.opencontainers.image.licenses=AGPL-3.0-only
COPY --from=build-go /build/bin/akvorado /usr/local/bin/akvorado
EXPOSE 8080
HEALTHCHECK --interval=20s CMD [ "/usr/local/bin/akvorado", "healthcheck" ]
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
