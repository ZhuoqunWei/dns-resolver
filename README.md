# Go DNS Resolver

[![CI](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml/badge.svg)](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml)

A small DNS server written in Go as part of a systems/networking learning project.

The current version can parse basic DNS query messages from raw bytes, listen for UDP DNS queries on `127.0.0.1:8053`, and serve one authoritative zone. It returns configured IPv4, IPv6, and SOA records loaded from `records.json`.

This is not a recursive resolver yet. It does not forward queries to upstream DNS servers, perform caching, or dynamically resolve real domain names.


## Current Server Behavior

The server currently supports:

* Reading two bytes safely as a `uint16`
* Parsing the fixed-size DNS header
* Extracting DNS flags from the 16-bit flags field
* Parsing a DNS QNAME from length-prefixed labels
* Parsing QTYPE and QCLASS
* Parsing one complete DNS query message with exactly one question
* Listening for UDP DNS queries on 127.0.0.1:8053
* Building a valid DNS response packet
* Returning configured A, AAAA, and SOA records for ClassIN
* Setting AA on responses for names inside the configured zone
* Returning NXDOMAIN with the zone SOA for missing in-zone names
* Returning NOERROR with the zone SOA for missing record types on existing names
* Returning REFUSED for names outside the configured zone
* Returning a valid empty response for unsupported query classes
* Returning clear errors for malformed or truncated input

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



## Running the UDP DNS Server

The server loads `records.json` once during startup. The file defines the zone origin, its SOA metadata, and address records:

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
The server listens on:

```
127.0.0.1:8053
```

`Port 8053` is used instead of port `53` because port `53` often requires elevated permissions, and port `5353` may already be used by system services such as mDNS.

Expected startup output:

```text
DNS UDP server listening on 127.0.0.1:8053
```

## Demo: A Query

Run:
```
dig +noedns @127.0.0.1 -p 8053 example.com A
```
This asks the local DNS server for the IPv4 address of example.com.

Expected behavior:
```
;; ->>HEADER<<- opcode: QUERY, status: NOERROR
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

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
dig +noedns @127.0.0.1 -p 8053 pool.example.com A
```

Expected answer section:

```text
pool.example.com.       90      IN      A       192.0.2.10
pool.example.com.       90      IN      A       192.0.2.11
```

Both records belong to one A RRset, so the response has `ANSWER: 2`. The server returns every configured member of the matching RRset.

## Demo: AAAA Query

Run:

`dig +noedns @127.0.0.1 -p 8053 example.com AAAA`

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
dig +noedns @127.0.0.1 -p 8053 example.com SOA
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

## UDP Response Truncation

Without EDNS, DNS-over-UDP responses are limited to 512 bytes. The response builder encodes resource records one at a time and appends only complete records that fit within that limit.

When required answer or authority data does not fit, the server:

- Stops before the next complete resource record.
- Sets `TC = 1`.
- Sets `ANCOUNT` and `NSCOUNT` to the records actually included.
- Sends a response no larger than 512 bytes.

A truncated response may contain part of an RRset, but clients should retry the query using a transport that permits a larger response. TCP fallback is not implemented yet, so this server currently provides only the correctly marked UDP response.

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

Additional behavior coverage includes:

- Packet handler rejects malformed queries
- Response builder does not set `RA`
- Response builder returns configured A, AAAA, and SOA answers for `ClassIN`
- Response builder returns every record in a matching RRset
- Response builder limits classic UDP responses to 512 bytes
- Oversized responses contain only complete resource records and set `TC`
- Truncated response counts match the records actually included
- Response builder sets AA for in-zone answers
- Response builder returns NXDOMAIN plus an SOA authority record for missing in-zone names
- Response builder returns NOERROR/NODATA plus an SOA authority record for missing types
- Response builder returns REFUSED for out-of-zone names
- General resource-record encoding rejects oversized RDATA
- Loopback UDP integration covers normal and truncated RRsets, SOA, NODATA, NXDOMAIN, and REFUSED
- JSON loader accepts valid RRsets and rejects duplicate data or inconsistent RRset TTLs

## Current Limitations

This project intentionally does not support everything yet.

**Current limitations:**

- Supports one question only
- Does not support compressed QNAMEs in incoming queries yet
- Does not support EDNS yet; use `+noedns` with `dig`
- UDP responses therefore use the classic 512-byte limit
- Does not provide TCP retry service for truncated responses yet
- Does not perform recursive resolution yet
- Does not forward queries to upstream DNS servers yet
- Does not implement caching yet
- Loads configuration only at startup; live reload is not supported yet
- Serves one configured zone
- Configurable address records are limited to A and AAAA

These limitations are intentional because the current milestone is focused on understanding DNS query parsing, UDP packet handling, and minimal DNS response construction.

## Running Tests

**Run:**

`go test -count=1 ./...`

Expected result:

`ok      github.com/zhuoqunwei/dns-resolver`
