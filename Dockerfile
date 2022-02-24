FROM golang:1.17-alpine AS build
RUN apk add --no-cache git make
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN make clean && make

# Do not use scratch, we use alpine to get an healthcheck
FROM alpine
COPY --from=build /app/bin/flowexporter /usr/local/bin/flowexporter
ENTRYPOINT [ "/usr/local/bin/flowexporter" ]
