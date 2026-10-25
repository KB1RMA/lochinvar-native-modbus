# syntax=docker/dockerfile:1
#
# Read-only Modbus RTU to MQTT bridge for a Lochinvar boiler. Build context: this directory (mqtt/).
#
#   docker run -d --restart unless-stopped --device /dev/ttyUSB0 \
#     -e MQTT_HOST=192.168.1.10 ghcr.io/kb1rma/lochinvar-native-modbus
#
# A static Go binary, cross-compiled on the build machine for each target platform (no emulation),
# on a distroless base with CA certificates (for MQTT over TLS) and no shell.

FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/lochinvar-mqtt ./cmd/lochinvar-mqtt

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/lochinvar-mqtt /usr/local/bin/lochinvar-mqtt
# Serial devices on Debian, Ubuntu and Raspberry Pi OS hosts belong to group dialout (gid 20). If the
# host's device has another group, pass --group-add <gid>.
USER 65532:20
ENTRYPOINT ["/usr/local/bin/lochinvar-mqtt"]
