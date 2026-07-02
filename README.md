# Go DNS Resolver

A small DNS parser written in Go as part of a systems/networking learning project.

The current version focuses on parsing a basic DNS query message from raw bytes. It does not send or receive network packets yet. The goal of this stage is to understand the DNS wire format before building UDP networking or DNS responses.

## Current Parser Behavior

The parser currently supports:

* Reading two bytes safely as a `uint16`
* Parsing the fixed-size DNS header
* Extracting DNS flags from the 16-bit flags field
* Parsing a DNS QNAME from length-prefixed labels
* Parsing QTYPE and QCLASS
* Parsing one complete DNS query message with exactly one question
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

## Tested Malformed Cases

The test suite checks that the parser handles invalid input safely.

Tested malformed cases include:

* Short DNS header
* `QDCOUNT = 0`
* `QDCOUNT = 2`
* Truncated QNAME label
* Missing QNAME terminating `00`
* Short QTYPE
* Short QCLASS
* Bad QNAME passed through `parseMessage`

## Current Limitations

This project intentionally does not support everything yet.

Current limitations:

* Supports one question only
* Does not support DNS compression pointers yet
* Does not listen on UDP yet
* Does not build DNS responses yet
* Does not perform recursive resolution yet
* Does not implement caching yet

These limitations are intentional because the current milestone is focused on understanding and testing DNS query parsing.

## Running Tests

Run:

```bash
go test -count=1 ./...
```

Expected result:

```text
ok      github.com/zhuoqunwei/dns-resolver
```

## Project Roadmap

Current completed milestone:

```text
Raw DNS query bytes -> Header -> Flags -> Question
```

Next planned milestones:

```text
1. Document and demo parser behavior
2. Add UDP listener
3. Receive a real query from dig
4. Parse real query bytes
5. Build a minimal DNS response
6. Return a hardcoded A record
7. Add logging and additional malformed packet handling
```
