# DNS Server Architecture

## Purpose

This project is a small DNS server written in Go to learn DNS wire-format parsing, UDP networking, and DNS response construction.

It is not a recursive resolver. It listens locally on `127.0.0.1:8053`, parses one DNS question, and returns a hardcoded `1.2.3.4` answer only for `A` queries in the `IN` class. Other query types receive a valid response with no answer records.

## Request and Response Flow

```text
dig
  |
  | UDP DNS query
  v
main.go: ReadFromUDP
  |
  | raw packet bytes
  v
dns.go: parseMessage
  |
  | parseHeader -> parseFlags -> parseQuestion
  v
Message { Header, Flags, Question }
  |
  v
response.go: buildResponse
  |
  | DNS response bytes
  v
main.go: WriteToUDP
  |
  v
dig prints the DNS response
```

For example, run the server in one terminal:

```bash
go run .
```

Then send an A query from another terminal:

```bash
dig +noedns @127.0.0.1 -p 8053 example.com A
```

The server receives the query, parses `example.com`, builds an answer for `1.2.3.4`, and sends the response back to `dig`.

## Runtime Flow

### 1. Receive a UDP packet

`main.go` creates a UDP listener on `127.0.0.1:8053`. It allocates a 512-byte buffer, then waits in a loop for packets with `ReadFromUDP`.

`ReadFromUDP` returns the number of bytes received and the sender address. The program slices the buffer to the exact packet length before parsing it:

```go
packet := buf[:n]
```

This prevents unused bytes from the fixed buffer from becoming part of the DNS message.

### 2. Parse the DNS query

`parseMessage` in `dns.go` converts the packet bytes into a `Message` struct. It expects exactly one question and does this work in order:

1. `parseHeader` reads the fixed 12-byte DNS header.
2. `parseFlags` extracts individual bits such as `QR`, `RD`, and `RA` from the header flags.
3. `parseQuestion` starts at byte offset 12, parses the QNAME, then reads QTYPE and QCLASS.

`parseQName` reads DNS labels such as `03 www 07 example 03 com 00` and joins them into `www.example.com`. It returns the offset immediately after the terminating `00`; `parseQuestion` uses that offset to locate the two-byte QTYPE and QCLASS fields.

The parser returns errors for truncated or malformed packets. `main.go` logs the error and continues waiting for the next UDP packet instead of terminating the server.

### 3. Build a DNS response

After a query is successfully parsed, `main.go` calls `buildResponse` in `response.go`.

`buildResponse` first finds the end of the original question section. It uses that location to read the query type and class, then constructs a new DNS response:

- Copies the transaction ID from the query so `dig` can match the reply to its request.
- Sets `QR = 1` to mark the packet as a response.
- Copies `RD` from the query.
- Leaves `RA = 0` because this server does not provide recursive resolution.
- Sets `QDCOUNT = 1` and copies the original question into the response.
- Sets `ANCOUNT = 1` only for `A / IN`; otherwise it sets `ANCOUNT = 0`.

For an `A / IN` query, the answer section contains:

```text
NAME     0xc00c  (pointer to the original QNAME at byte 12)
TYPE     1       (A)
CLASS    1       (IN)
TTL      60
RDLENGTH 4
RDATA    1.2.3.4
```

The `0xc00c` name uses DNS compression in the response. It points back to the QNAME in the copied question section instead of encoding the domain name again.

### 4. Send the response

`main.go` sends the bytes returned by `buildResponse` to the original sender address with `WriteToUDP`. `dig` receives the response and displays either:

- One `A` answer containing `1.2.3.4` for `A / IN` queries.
- A valid DNS response with `ANSWER: 0` for unsupported types, such as `AAAA`.

## File Responsibilities

| File | Responsibility |
| --- | --- |
| `main.go` | Opens the UDP socket, receives packets, calls the parser and response builder, logs results, and sends replies. |
| `dns.go` | Parses DNS headers, flags, QNAMEs, questions, and one complete DNS message. |
| `response.go` | Builds the DNS response header, copies the question, and optionally appends the hardcoded A record. |
| `dns_test.go` | Tests parser behavior and malformed DNS query handling. |
| `response_test.go` | Tests response flags, answer counts, answer bytes, and unsupported query behavior. |
| `README.md` | Provides setup instructions, `dig` demos, DNS wire-format background, and limitations. |

## Current Behavior

| Query | Result |
| --- | --- |
| `A / IN` | Returns one hardcoded answer: `1.2.3.4` with TTL 60. |
| `AAAA / IN` | Returns a valid response with no answers. |
| Other type or class | Returns a valid response with no answers. |
| Malformed packet | Logs a parse or response-building error and keeps the UDP server running. |

The current A response is based only on QTYPE and QCLASS. The queried domain name is not yet used to select a record, so every valid `A / IN` query receives `1.2.3.4`.

## Design Decisions

- The parser is separate from UDP networking so DNS byte handling can be tested without starting a server.
- The response builder is separate from `main.go` so response bytes can be tested directly.
- The server accepts one question per message to keep the first implementation understandable.
- The server reports `RA = 0` because it does not recursively resolve or forward queries.
- The response uses compression only when writing the answer. Incoming compressed QNAMEs are not supported yet.

## Current Limitations

- One question per message only.
- No compressed QNAMEs in incoming queries.
- No EDNS support; use `+noedns` with `dig` for the documented demo.
- UDP only; no TCP DNS fallback.
- No recursive resolution, upstream forwarding, caching, or dynamic record lookup.
- `A / IN` responses are hardcoded to `1.2.3.4` regardless of the queried domain name.

## Verification

Run the unit tests:

```bash
go test -count=1 ./...
```

Run the server and use the `dig` commands in `README.md` to verify the full UDP request/response path.
