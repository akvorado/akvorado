FROM squidfunk/mkdocs-material:8.2.5 AS documentation
COPY mkdocs.yml /docs/
COPY docs /docs/docs/
RUN mkdocs build --strict --site-dir /output

FROM golang:1.17-alpine AS build
RUN apk add --no-cache git make gcc musl-dev protoc
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN make clean
COPY --from=documentation /output web/data/
RUN find web/data
RUN make test && make

# Do not use scratch, we use alpine to get an healthcheck
FROM alpine
RUN apk add --no-cache shared-mime-info
COPY --from=build /app/bin/akvorado /usr/local/bin/akvorado
HEALTHCHECK CMD wget -Y off -q -O - http://localhost:8080/api/v0/healthcheck || exit 1
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
