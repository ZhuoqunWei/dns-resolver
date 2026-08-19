# Release Checklist

## Version 1 Scope

Version 1 is a small authoritative DNS server, not a recursive resolver. The release includes:

- JSON-configured A, AAAA, SOA, and RRset data for one zone.
- Authoritative positive, NXDOMAIN, NODATA, and REFUSED responses.
- UDP, TCP, EDNS payload negotiation, and complete-record truncation.
- TCP connection deadlines and graceful process shutdown.
- Configurable listen and zone-file paths.
- A static non-root container image with automated smoke coverage.

Recursive resolution, caching, upstream forwarding, live reload, and multiple zones are outside this release.

## Acceptance

Run the Go checks:

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test -race -count=1 ./...
```

Run the container checks with Docker running and `dig` installed:

```bash
./scripts/container-smoke.sh dns-resolver:release
```

Review the final worktree:

```bash
git status
git log -1 --oneline
```

## Tag and Publish

After the release commit is on `main`, create and push the annotated tag:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

Create the GitHub release from that tag:

```bash
gh release create v1.0.0 \
  --title "Go Authoritative DNS Server v1.0.0" \
  --notes "First complete authoritative-server milestone with UDP, TCP, EDNS, configurable zone data, graceful shutdown, and container delivery."
```
