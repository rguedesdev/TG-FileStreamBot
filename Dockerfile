FROM --platform=$BUILDPLATFORM golang:1.25.0-alpine3.21 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /app/fsb -ldflags="-w -s" ./cmd/fsb \
    && mkdir -p /runtime/logs /runtime/sessions

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder --chown=65532:65532 /app/fsb /app/fsb
COPY --from=builder --chown=65532:65532 /runtime/logs/ /app/logs/
COPY --from=builder --chown=65532:65532 /runtime/sessions/ /app/sessions/
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/fsb", "run"]
