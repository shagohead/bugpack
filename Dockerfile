# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod  go mod download

COPY bugpack bugpack/
RUN \
	--mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /go/bin/bugpack bugpack/*.go

FROM alpine
COPY --from=builder /go/bin/bugpack /usr/local/bin/bugpack
ENTRYPOINT ["bugpack"]
LABEL org.opencontainers.image.authors="Vakhmin Anton <html.ru@gmail.com>"
