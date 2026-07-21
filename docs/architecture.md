# DNS Server Architecture

## Purpose

This project is a small DNS server written in Go to learn DNS wire-format parsing, UDP networking, and DNS response construction.

It is not a recursive resolver. It listens locally on `127.0.0.1:8053`, parses one DNS question, and returns configured IPv4 answers for `A` queries in the `IN` class. Unknown names return `NXDOMAIN`, while unsupported types on configured names receive a valid response with no answer records.

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
  | parsed message and records
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

`buildResponse` receives the parsed `Message` and the record map from `main.go`. It encodes a new question section and constructs the DNS response entirely from the parsed fields:

- Copies the transaction ID from the query so `dig` can match the reply to its request.
- Sets `QR = 1` to mark the packet as a response.
- Copies `RD` from the query.
- Leaves `RA = 0` because this server does not provide recursive resolution.
- Sets `QDCOUNT = 1` and copies the original question into the response.
- Sets `ANCOUNT = 1` only when the queried name has a configured `A / IN` record; otherwise it sets `ANCOUNT = 0`.
- Sets `RCODE = NXDOMAIN` when the queried name is not configured.

For the configured `example.com A / IN` query, the answer section contains:

```text
NAME     0xc00c  (pointer to the original QNAME at byte 12)
TYPE     1       (A)
CLASS    1       (IN)
TTL      60
RDLENGTH 4
RDATA    1.2.3.4
```

The `0xc00c` name uses DNS compression in the response. It points back to the encoded QNAME at byte 12 instead of encoding the domain name again in the answer.

### 4. Send the response

`main.go` sends the bytes returned by `buildResponse` to the original sender address with `WriteToUDP`. `dig` receives the response and displays either:

- The configured `A` answer for a known name.
- `NXDOMAIN` for an unknown name.
- A valid DNS response with `ANSWER: 0` for unsupported types on known names, such as `example.com AAAA`.

## File Responsibilities

| File | Responsibility |
| --- | --- |
| `main.go` | Owns the configured records, opens the UDP socket, receives packets, calls the parser and response builder, logs results, and sends replies. |
| `dns.go` | Parses DNS headers, flags, QNAMEs, questions, and one complete DNS message. |
| `response.go` | Encodes a DNS response from a parsed message and explicitly supplied records, including the question and optional A answer. |
| `dns_test.go` | Tests parser behavior and malformed DNS query handling. |
| `response_test.go` | Tests response flags, answer counts, answer bytes, and unsupported query behavior. |
| `README.md` | Provides setup instructions, `dig` demos, DNS wire-format background, and limitations. |

## Current Behavior

| Query | Result |
| --- | --- |
| `example.com A / IN` | Returns `1.2.3.4` with TTL 60. |
| `test.local A / IN` | Returns `5.6.7.8` with TTL 60. |
| Unknown name | Returns `NXDOMAIN` with no answers. |
| Unsupported type on a configured name | Returns `NOERROR` with no answers. |
| Unsupported class | Returns a valid response with no answers. |
| Malformed packet | Logs a parse or response-building error and keeps the UDP server running. |

The current A response is selected by QNAME, QTYPE, and QCLASS from an in-memory record map owned by `main.go`.

## Design Decisions

- The parser is separate from UDP networking so DNS byte handling can be tested without starting a server.
- The response builder is separate from `main.go` so response bytes can be tested directly.
- Each packet is parsed into a `Message` once, and that parsed message is passed to the response builder.
- The response builder depends on parsed data rather than the original query bytes.
- The record map is passed explicitly to the response builder instead of being read as global state.
- The server accepts one question per message to keep the first implementation understandable.
- The server reports `RA = 0` because it does not recursively resolve or forward queries.
- The response uses compression only when writing the answer. Incoming compressed QNAMEs are not supported yet.

## Current Limitations

- One question per message only.
- No compressed QNAMEs in incoming queries.
- No EDNS support; use `+noedns` with `dig` for the documented demo.
- UDP only; no TCP DNS fallback.
- No recursive resolution, upstream forwarding, or caching.
- Records are stored in code rather than loaded from an external configuration file.

## Verification

Run the unit tests:

```bash
go test -count=1 ./...
```

Run the server and use the `dig` commands in `README.md` to verify the full UDP request/response path.
