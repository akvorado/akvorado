FROM --platform=$BUILDPLATFORM node:20-alpine AS build-js
RUN apk add --no-cache make
WORKDIR /build
COPY console/frontend console/frontend
COPY Makefile .
RUN make console/data/frontend

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build-go
RUN apk add --no-cache make mailcap curl
WORKDIR /build
# Cache for modules
COPY go.mod go.sum .
RUN go mod download
# Build
COPY . .
COPY --from=build-js /build/console/data/frontend console/data/frontend
RUN touch console/frontend/node_modules console/data/frontend .fmt-js~ .fmt.go~ .lint-js~ .lint-go~
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION
RUN make

FROM gcr.io/distroless/static:latest
LABEL org.opencontainers.image.source=https://github.com/akvorado/akvorado
LABEL org.opencontainers.image.description="Akvorado: flow collector, enricher and visualizer"
LABEL org.opencontainers.image.licenses=AGPL-3.0-only
COPY --from=build-go /etc/mime.types /etc/mime.types
COPY --from=build-go /build/bin/akvorado /usr/local/bin/akvorado
EXPOSE 8080
HEALTHCHECK --interval=20s CMD [ "/usr/local/bin/akvorado", "healthcheck" ]
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
