# syntax=docker/dockerfile:1

FROM golang:1.24.1-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/dns-resolver \
    .

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/ZhuoqunWei/dns-resolver"
LABEL org.opencontainers.image.description="Small authoritative DNS server written in Go"

COPY --from=build --chown=65532:65532 /out/dns-resolver /dns-resolver
COPY --chown=65532:65532 records.json /etc/dns-resolver/records.json

USER 65532:65532

EXPOSE 8053/udp
EXPOSE 8053/tcp

ENTRYPOINT ["/dns-resolver"]
CMD ["-listen", "0.0.0.0:8053", "-config", "/etc/dns-resolver/records.json"]
