FROM golang:1.18-alpine AS build
RUN apk add --no-cache git make gcc musl-dev protoc shared-mime-info yarn curl
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN make clean && make

# Do not use scratch, we use alpine to get an healthcheck
FROM alpine
RUN apk add --no-cache shared-mime-info
COPY --from=build /app/bin/akvorado /usr/local/bin/akvorado
EXPOSE 8080
HEALTHCHECK CMD wget -Y off -q -O - http://localhost:8080/api/v0/healthcheck || exit 1
ENTRYPOINT [ "/usr/local/bin/akvorado" ]
