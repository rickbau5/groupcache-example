FROM golang:1.17-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /build/groupcache-example

FROM alpine:latest

WORKDIR /

COPY --from=build /build/groupcache-example /groupcache-example

ENTRYPOINT ["/groupcache-example"]
