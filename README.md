# Go Authoritative DNS Server

[![CI](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml/badge.svg)](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml)

A compact authoritative DNS server implemented in Go without a DNS framework. It parses and encodes DNS wire messages, serves a JSON-configured zone over UDP and TCP, supports EDNS-aware truncation, and shuts down cleanly under process signals.

The project intentionally does not perform recursive resolution, upstream forwarding, or caching. Its scope is a small, testable authoritative server whose protocol and transport behavior can be explained end to end. See the [architecture document](docs/architecture.md) for the request flow and design decisions.

## Quick Start

Start the server from source:

```bash
go run .
```

Or run the non-root container:

```bash
docker build -t dns-resolver:local .
docker run --rm -p 8053:8053/udp -p 8053:8053/tcp dns-resolver:local
```

Then, from another terminal, the configured query returns `1.2.3.4`:

```bash
dig +noedns +short @127.0.0.1 -p 8053 example.com A
```

## Feature Matrix

| Area | Implemented behavior |
| --- | --- |
| Wire format | DNS headers, flags, QNAME, QTYPE/QCLASS, resource records, and EDNS OPT metadata |
| Zone data | Validated JSON configuration with A, AAAA, SOA, and multi-value RRsets |
| Authority | Authoritative answers, NXDOMAIN, NOERROR/NODATA, REFUSED, and negative SOA records |
| UDP | Classic 512-byte responses, EDNS negotiation up to 1232 bytes, and whole-record truncation |
| TCP | Length-prefixed framing, concurrent clients, connection reuse, and read/write deadlines |
| Lifecycle | Configurable startup options plus graceful `SIGINT`/`SIGTERM` shutdown |
| Delivery | Static non-root container image and GitHub Actions container smoke testing |

The main parser function is:

```go
parseMessage(data []byte) (Message, error)
```

It returns a `Message` containing:

```go
type Message struct {
    Header   Header
    Flags    Flags
    Question Question
    EDNS     *EDNS
}
```

## DNS Header Layout

A DNS message starts with a fixed 12-byte header.

This project defines:

```go
const HeaderSize = 12
```

The DNS header contains six 2-byte fields:

```text
Bytes 0-1    ID
Bytes 2-3    Flags
Bytes 4-5    QDCOUNT
Bytes 6-7    ANCOUNT
Bytes 8-9    NSCOUNT
Bytes 10-11  ARCOUNT
```

Each field is stored in network byte order, also known as big-endian order.

For example:

```text
0x12 0x34
```

is parsed as:

```text
0x1234 = 4660
```

The helper function `readU16` reads two bytes starting at an offset and returns a `uint16`.

## DNS Flags

The parser breaks the 16-bit flags field into:

```go
type Flags struct {
    QR     bool
    Opcode uint8
    AA     bool
    TC     bool
    RD     bool
    RA     bool
    Z      uint8
    RCode  uint8
}
```

Examples:

* `RD` means recursion desired
* `QR` tells whether the message is a query or response
* `RCode` stores the response code
* `AA` means authoritative answer
* `TC` means truncated response
* `RA` means recursion available

## DNS Question Layout

After the 12-byte header, a DNS query usually contains a question section.

This parser currently supports one question only.

A question contains:

```text
QNAME   variable length
QTYPE   2 bytes
QCLASS  2 bytes
```

### QNAME

DNS names are not stored as plain strings. They are stored as length-prefixed labels.

For example:

```text
www.example.com
```

is encoded as:

```text
03 77 77 77
07 65 78 61 6d 70 6c 65
03 63 6f 6d
00
```

This means:

```text
03                 length = 3
77 77 77           "www"

07                 length = 7
65 78 61 6d 70 6c 65   "example"

03                 length = 3
63 6f 6d           "com"

00                 end of name
```

The parser turns those bytes into:

```text
www.example.com
```

It also returns the next offset after the terminating `00`, so the parser knows where to read QTYPE and QCLASS.

### QTYPE and QCLASS

After QNAME, the parser reads:

```text
QTYPE  = 2 bytes
QCLASS = 2 bytes
```

This project defines:

```go
const (
    TypeA   uint16 = 1
    ClassIN uint16 = 1
)
```

So:

```text
QTYPE = 1
```

means an `A` record query.

```text
QCLASS = 1
```

means Internet class, usually written as `IN`.

## Example Raw DNS Query

This is an example DNS query for:

```text
www.example.com A IN
```

Raw bytes:

```go
[]byte{
    // Header
    0x12, 0x34, // ID
    0x01, 0x00, // Flags: recursion desired
    0x00, 0x01, // QDCOUNT: 1 question
    0x00, 0x00, // ANCOUNT
    0x00, 0x00, // NSCOUNT
    0x00, 0x00, // ARCOUNT

    // QNAME: www.example.com
    0x03, 'w', 'w', 'w',
    0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
    0x03, 'c', 'o', 'm',
    0x00,

    // QTYPE and QCLASS
    0x00, 0x01, // QTYPE: A
    0x00, 0x01, // QCLASS: IN
}
```

The parser should return:

```text
Header.ID = 0x1234
Header.QDCount = 1
Flags.RD = true
Question.Name = "www.example.com"
Question.QType = 1
Question.QClass = 1
```



## Running the DNS Server

By default, the server loads `records.json` once during startup. The file defines the zone origin, its SOA metadata, and address records:

```json
{
  "origin": "example.com",
  "soa": {
    "nameServer": "ns1.example.com",
    "responsibleName": "hostmaster.example.com",
    "serial": 2026072501,
    "refresh": 3600,
    "retry": 600,
    "expire": 86400,
    "minimum": 300,
    "ttl": 300
  },
  "records": [
    {
      "name": "example.com",
      "type": "A",
      "value": "1.2.3.4",
      "ttl": 60
    },
    {
      "name": "example.com",
      "type": "AAAA",
      "value": "2001:db8::1",
      "ttl": 120
    },
    {
      "name": "pool.example.com",
      "type": "A",
      "value": "192.0.2.10",
      "ttl": 90
    },
    {
      "name": "pool.example.com",
      "type": "A",
      "value": "192.0.2.11",
      "ttl": 90
    }
  ]
}
```

The loader converts A, AAAA, and SOA values into wire-ready RDATA. Repeated owner-and-type entries form an RRset, such as the two A records for `pool.example.com`. Every member of an RRset must have distinct RDATA and the same TTL.

The origin defines the zone served by this process. Names and type strings are normalized for case-insensitive lookup. Invalid names, unsupported types, address-family mismatches, duplicate values, mixed TTLs within an RRset, and records outside the configured zone prevent the server from starting.

Start the server:

```
go run .
```

The default options are:

```text
-listen 127.0.0.1:8053
-config records.json
```

Override either value when starting the server:

```bash
go run . -listen 127.0.0.1:9053 -config ./records.json
```

UDP and TCP bind to the same resolved address. The server loads the selected configuration file before opening either listener.

To use a custom zone in Docker, mount the file read-only at the container's default configuration path:

```bash
docker run --rm \
  -p 8053:8053/udp \
  -p 8053:8053/tcp \
  -v "$PWD/records.json:/etc/dns-resolver/records.json:ro" \
  dns-resolver:local
```

`Port 8053` is used instead of port `53` because port `53` often requires elevated permissions, and port `5353` may already be used by system services such as mDNS.

Expected startup output:

```text
DNS server listening on 127.0.0.1:8053 over UDP and TCP
```

Stop the server with `Ctrl+C`. On `SIGINT` or `SIGTERM`, the process closes both listeners, closes active TCP connections, waits for the transport goroutines to finish, and prints:

```text
DNS server stopped
```

## Demo: A Query

Run:
```
dig @127.0.0.1 -p 8053 example.com A
```
This asks the local DNS server for the IPv4 address of example.com.

Expected behavior:
```
;; ->>HEADER<<- opcode: QUERY, status: NOERROR
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1
;; WARNING: recursion requested but not available

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232

;; QUESTION SECTION:
;example.com.                   IN      A

;; ANSWER SECTION:
example.com.            60      IN      A       1.2.3.4
```
The A response is loaded from `records.json` into the zone's generic runtime record store. Each `Record` stores a TTL and validated, wire-ready RDATA:

```
example.com.  60  IN  A  1.2.3.4
```
This does not mean the server performed a real DNS lookup. It means the server found `example.com` in its configured records and returned the matching IPv4 address.

## Demo: Multiple A Records

Run:

```bash
dig @127.0.0.1 -p 8053 pool.example.com A
```

Expected answer section:

```text
pool.example.com.       90      IN      A       192.0.2.10
pool.example.com.       90      IN      A       192.0.2.11
```

Both records belong to one A RRset, so the response has `ANSWER: 2`. The server returns every configured member of the matching RRset.

## Demo: AAAA Query

Run:

`dig @127.0.0.1 -p 8053 example.com AAAA`

This asks the local DNS server for the IPv6 address of example.com.

Expected behavior:
```
;; ->>HEADER<<- opcode: QUERY, status: NOERROR
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;example.com.                   IN      AAAA

;; ANSWER SECTION:
example.com.            120     IN      AAAA    2001:db8::1
```

The same general resource-record encoder writes both A and AAAA answers. The configured RDATA determines whether the payload is four or sixteen bytes.

Query the zone metadata directly with:

```bash
dig @127.0.0.1 -p 8053 example.com SOA
```

The response contains the configured SOA in the answer section.

## Response Behavior

Current response behavior:
```
example.com A      -> NOERROR, ANSWER: 1, 1.2.3.4
example.com AAAA   -> NOERROR, ANSWER: 1, 2001:db8::1
example.com SOA    -> NOERROR, ANSWER: 1
test.example.com A -> NOERROR, ANSWER: 1, 5.6.7.8
pool.example.com A -> NOERROR, ANSWER: 2, 192.0.2.10 and 192.0.2.11
missing.example.com A -> NXDOMAIN, ANSWER: 0, AUTHORITY: SOA
example.com TXT    -> NOERROR, ANSWER: 0, AUTHORITY: SOA
other.com A        -> REFUSED, ANSWER: 0, AUTHORITY: 0
```
An unknown name inside the configured zone returns NXDOMAIN. A configured name queried for a missing type returns NOERROR/NODATA. Both negative responses include the zone SOA in the authority section so clients can cache the result. The negative SOA TTL is the smaller of the SOA TTL and MINIMUM values. A name outside the zone returns REFUSED without claiming authority.

Every response uses the same transaction ID as the query, sets QR, copies RD, and leaves RA unset. Responses for the configured zone also set AA.

The answer section uses DNS name compression:

`0xc00c`

This points back to byte offset 12, where the original QNAME starts in the question section.

## UDP Response Sizing and Truncation

Without EDNS, DNS-over-UDP responses are limited to 512 bytes. An EDNS(0) query advertises the largest UDP payload the client can receive in the OPT record's CLASS field. The server uses the advertised value with a lower bound of 512 bytes and an upper server cap of 1232 bytes, following the negotiation model in [RFC 6891](https://www.rfc-editor.org/rfc/rfc6891.html).

```text
response limit = min(max(client advertised size, 512), 1232)
```

The server includes an OPT record in every response to an EDNS request and reserves that record's 11 bytes before selecting answers. The response builder encodes resource records one at a time and appends only complete records that fit within the resulting limit.

When required answer or authority data does not fit, the server:

- Stops before the next complete resource record.
- Sets `TC = 1`.
- Sets `ANCOUNT` and `NSCOUNT` to the records actually included.
- Sends a response no larger than the negotiated limit.

A truncated response may contain part of an RRset. A client can retry the query over TCP to receive the complete response. Use `+noedns` with `dig` to exercise the legacy 512-byte path explicitly.

The server also listens for DNS over TCP on the same address. Each TCP DNS message uses a two-byte, big-endian length prefix. A connection can carry multiple queries, and different client connections are handled concurrently.

Each connection has a 10-second read deadline for receiving one complete framed query and a 5-second write deadline for sending its response. The read deadline is refreshed before each query on a reused connection. Idle clients, incomplete frames, slow senders, and clients that stop reading responses therefore release their connection and handler goroutine instead of blocking indefinitely.

Force a TCP query with:

```bash
dig +tcp @127.0.0.1 -p 8053 pool.example.com A
```

TCP responses use the complete selected RRset when it fits within the protocol's 65,535-byte message limit. The integration tests configure a 40-record RRset and verify three paths: classic UDP truncates at 512 bytes, EDNS UDP returns all 40 records within 1232 bytes, and TCP returns all 40 records without `TC`.

## Tested Malformed Cases

The test suite checks that the parser and response builder handle invalid input safely.

Tested malformed cases include:
- Short DNS header
- `QDCOUNT = 0`
- `QDCOUNT = 2`
- Truncated QNAME label
- Missing QNAME terminating `00`
- Short QTYPE
- Short QCLASS
- Bad QNAME passed through `parseMessage`
- Truncated or misplaced OPT records
- Multiple OPT records in one query
- Undeclared trailing bytes

Additional behavior coverage includes:

- Packet handler rejects malformed queries
- Response builder does not set `RA`
- Response builder returns configured A, AAAA, and SOA answers for `ClassIN`
- Response builder returns every record in a matching RRset
- Response builder limits classic UDP responses to 512 bytes
- Oversized responses contain only complete resource records and set `TC`
- Truncated response counts match the records actually included
- EDNS UDP limits honor the client advertisement and the 1232-byte server cap
- EDNS responses include an OPT record without exceeding the negotiated size
- Unsupported EDNS versions return `BADVERS`
- TCP messages use a two-byte length prefix and tolerate fragmented stream reads
- Malformed and incomplete TCP frames are rejected
- One TCP connection can carry multiple DNS queries
- TCP read deadlines close idle and partially sending clients
- TCP write deadlines close clients that stop reading responses
- Read and write deadlines are refreshed for every query on a reused connection
- An idle TCP client does not block queries from another connection
- Cancellation stops both transports and closes active TCP connections
- An unexpected failure in one transport stops the other transport
- Oversized UDP RRsets are returned completely over TCP without `TC`
- Response builder sets AA for in-zone answers
- Response builder returns NXDOMAIN plus an SOA authority record for missing in-zone names
- Response builder returns NOERROR/NODATA plus an SOA authority record for missing types
- Response builder returns REFUSED for out-of-zone names
- General resource-record encoding rejects oversized RDATA
- Loopback UDP and TCP integration covers normal, EDNS-sized, and truncated RRsets, SOA, NODATA, NXDOMAIN, and REFUSED
- JSON loader accepts valid RRsets and rejects duplicate data or inconsistent RRset TTLs

## Current Limitations

This project intentionally does not support everything yet.

**Current limitations:**

- Supports one question only
- Does not support compressed QNAMEs in incoming queries
- EDNS support is limited to version 0 payload-size negotiation; DNSSEC data and other EDNS options are not implemented
- UDP responses are limited to 512 bytes without EDNS and 1232 bytes with EDNS
- Does not perform recursive resolution
- Does not forward queries to upstream DNS servers
- Does not implement caching
- Loads configuration only at startup; live reload is not supported
- Serves one configured zone
- Configurable address records are limited to A and AAAA

These limitations are intentional because the current milestone is focused on authoritative DNS behavior, transport handling, and DNS response construction.

## Running Tests

**Run:**

`go test -count=1 ./...`

Expected result:

`ok      github.com/zhuoqunwei/dns-resolver`

Run the same container acceptance test used by CI:

```bash
./scripts/container-smoke.sh dns-resolver:smoke
```

The script builds the image, verifies positive UDP and TCP answers, checks NXDOMAIN and REFUSED responses, and confirms graceful container shutdown. The [release checklist](docs/release.md) contains the complete `v1.0.0` acceptance and tagging procedure.
