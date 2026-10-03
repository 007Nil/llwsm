# Build stage
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X llwsm/internal/version.Version=0.1.0" \
    -o /out/sysmon ./cmd/sysmon

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && mkdir -p /host
COPY --from=build /out/sysmon /usr/local/bin/sysmon
EXPOSE 8090
# Monitor the host: mount the host root filesystem read-only at /host
# (see docker-compose.yml). /host/proc and /host/sys then expose host
# kernel state, and host mount points are reachable under /host.
ENTRYPOINT ["/usr/local/bin/sysmon"]
CMD ["--root", "/host", "--listen", "0.0.0.0:8090"]
