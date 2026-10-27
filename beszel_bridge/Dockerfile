ARG BUILD_FROM=ghcr.io/home-assistant/base:latest
FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go compatibility.json ./
ARG TARGETARCH
ARG BUILD_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${BUILD_VERSION}" -o /out/bridge .

FROM $BUILD_FROM
ARG BUILD_VERSION=dev
LABEL io.hass.version="${BUILD_VERSION}" \
    io.hass.type="app" \
    org.opencontainers.image.version="${BUILD_VERSION}" \
    org.opencontainers.image.source="https://github.com/vineetchoudhary/home-assistant-beszel-agent"
COPY --from=builder /out/bridge /usr/local/bin/bridge
COPY run.sh /run.sh
RUN chmod +x /run.sh
HEALTHCHECK --interval=60s --timeout=5s --start-period=10s CMD ["/usr/local/bin/bridge", "health"]
CMD ["/run.sh"]
