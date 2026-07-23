# Go DNS Resolver

[![CI](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml/badge.svg)](https://github.com/ZhuoqunWei/dns-resolver/actions/workflows/ci.yml)

A small DNS server written in Go as part of a systems/networking learning project.

The current version can parse basic DNS query messages from raw bytes, listen for UDP DNS queries on `127.0.0.1:8053`, and return a minimal DNS response. For configured `A / IN` queries, it returns the matching IPv4 address from an in-memory record map.

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
* Returning configured A records for TypeA / ClassIN
* Returning NXDOMAIN for names that are not configured
* Returning a valid response with ANCOUNT = 0 for unsupported query types or classes on configured names
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

Start the server:
```
go run .
```
The server listens on:

```
127.0.0.1:8053
```

`Port 8053` is used instead of port `53` because port `53` often requires elevated permissions, and port `5353` may already be used by system services such as mDNS.
```
Expected startup output:

DNS UDP server listening on `127.0.0.1:8053`
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
;; flags: qr rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;example.com.                   IN      A

;; ANSWER SECTION:
example.com.            60      IN      A       1.2.3.4
```
The A response is currently configured in the in-memory record map:

```
example.com.  60  IN  A  1.2.3.4
```
This does not mean the server performed a real DNS lookup. It means the server found `example.com` in its configured records and returned the matching IPv4 address.

## Demo: AAAA Query

Run:

`dig +noedns @127.0.0.1 -p 8053 example.com AAAA`

This asks the local DNS server for the IPv6 address of example.com.

Expected behavior:
```
;; ->>HEADER<<- opcode: QUERY, status: NOERROR
;; flags: qr rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;example.com.                   IN      AAAA
```
AAAA queries are unsupported for now, so the server returns a valid DNS response with:

ANSWER: `0`

This confirms that unsupported query types do not receive fake answers.

## Response Behavior

Current response behavior:
```
example.com A      -> NOERROR, ANSWER: 1, 1.2.3.4
test.local A       -> NOERROR, ANSWER: 1, 5.6.7.8
other.com A        -> NXDOMAIN, ANSWER: 0
example.com AAAA   -> NOERROR, ANSWER: 0
```
An unknown name returns NXDOMAIN. A configured name queried for an unsupported record type returns NOERROR with an empty answer section. Every response uses the same transaction ID as the query, sets QR = true, copies RD from the query, and sets RA = false.

The answer section uses DNS name compression:

`0xc00c`

This points back to byte offset 12, where the original QNAME starts in the question section.

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
- Response builder returns configured answers only for `TypeA / ClassIN`
- Response builder returns NXDOMAIN for unknown names
- Response builder returns NOERROR with `ANCOUNT = 0` for unsupported query types on configured names
- Loopback UDP integration for configured and unknown-name queries

## Current Limitations

This project intentionally does not support everything yet.

**Current limitations:**

- Supports one question only
- Does not support compressed QNAMEs in incoming queries yet
- Does not support EDNS yet; use `+noedns` with `dig`
- Does not perform recursive resolution yet
- Does not forward queries to upstream DNS servers yet
- Does not implement caching yet
- Stores A records in a hardcoded in-memory map rather than an external configuration file

These limitations are intentional because the current milestone is focused on understanding DNS query parsing, UDP packet handling, and minimal DNS response construction.

## Running Tests

**Run:**

`go test -count=1 ./...`

Expected result:

`ok      github.com/zhuoqunwei/dns-resolver`
