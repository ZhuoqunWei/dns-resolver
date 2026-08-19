#!/usr/bin/env bash

set -euo pipefail

image="${1:-dns-resolver:smoke}"
container_name="${DNS_CONTAINER_NAME:-dns-resolver-smoke}"
host_port="${DNS_HOST_PORT:-18053}"

cleanup() {
	docker rm --force "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

command -v docker >/dev/null || {
	echo "docker is required" >&2
	exit 1
}
command -v dig >/dev/null || {
	echo "dig is required" >&2
	exit 1
}

cleanup
docker build --tag "$image" .
docker run \
	--detach \
	--name "$container_name" \
	--publish "127.0.0.1:${host_port}:8053/udp" \
	--publish "127.0.0.1:${host_port}:8053/tcp" \
	"$image" >/dev/null

udp_answer=""
for _ in {1..20}; do
	udp_answer="$(dig +noedns +short +time=1 +tries=1 @127.0.0.1 -p "$host_port" example.com A 2>/dev/null || true)"
	if [[ "$udp_answer" == "1.2.3.4" ]]; then
		break
	fi
	sleep 0.25
done

if [[ "$udp_answer" != "1.2.3.4" ]]; then
	echo "UDP answer = '$udp_answer', want '1.2.3.4'" >&2
	docker logs "$container_name" >&2
	exit 1
fi

tcp_answer="$(dig +tcp +noedns +short +time=1 +tries=1 @127.0.0.1 -p "$host_port" example.com A)"
if [[ "$tcp_answer" != "1.2.3.4" ]]; then
	echo "TCP answer = '$tcp_answer', want '1.2.3.4'" >&2
	exit 1
fi

nxdomain_response="$(dig +noedns +time=1 +tries=1 @127.0.0.1 -p "$host_port" missing.example.com A)"
if [[ "$nxdomain_response" != *"status: NXDOMAIN"* ]]; then
	echo "missing.example.com did not return NXDOMAIN" >&2
	exit 1
fi

refused_response="$(dig +noedns +time=1 +tries=1 @127.0.0.1 -p "$host_port" other.com A)"
if [[ "$refused_response" != *"status: REFUSED"* ]]; then
	echo "other.com did not return REFUSED" >&2
	exit 1
fi

docker stop --time 5 "$container_name" >/dev/null
shutdown_logs="$(docker logs "$container_name")"
if [[ "$shutdown_logs" != *"DNS server stopped"* ]]; then
	echo "container did not report a graceful shutdown" >&2
	exit 1
fi

docker rm "$container_name" >/dev/null
trap - EXIT

echo "Container smoke test passed on UDP and TCP port $host_port"
