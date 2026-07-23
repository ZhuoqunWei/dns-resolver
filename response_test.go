package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func sampleQueryWithTypeClass(qtype uint16, qclass uint16) []byte {
	query := []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: example.com
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE + QCLASS placeholder
		0x00, 0x00,
		0x00, 0x00,
	}

	binary.BigEndian.PutUint16(query[len(query)-4:len(query)-2], qtype)
	binary.BigEndian.PutUint16(query[len(query)-2:], qclass)

	return query
}

func sampleOtherDomainAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: other.com
		0x05, 'o', 't', 'h', 'e', 'r',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE: A, QCLASS: IN
		0x00, 0x01,
		0x00, 0x01,
	}
}

func sampleTestLocalAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: test.local
		0x04, 't', 'e', 's', 't',
		0x05, 'l', 'o', 'c', 'a', 'l',
		0x00,

		// QTYPE: A, QCLASS: IN
		0x00, 0x01,
		0x00, 0x01,
	}
}

func buildTestResponse(t *testing.T, query []byte) []byte {
	t.Helper()

	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	records := map[string][4]byte{
		"example.com": {1, 2, 3, 4},
		"test.local":  {5, 6, 7, 8},
	}

	response, err := buildResponse(msg, records)
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}

	return response
}

func TestEncodeQName(t *testing.T) {
	tests := []struct {
		name    string
		qname   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "encodes example.com",
			qname: "example.com",
			want: []byte{
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
			},
		},
		{
			name:  "encodes root",
			qname: "",
			want:  []byte{0x00},
		},
		{
			name:    "rejects empty label",
			qname:   "example..com",
			wantErr: true,
		},
		{
			name:    "rejects label longer than 63 bytes",
			qname:   strings.Repeat("a", 64) + ".com",
			wantErr: true,
		},
		{
			name:    "rejects encoded name longer than 255 bytes",
			qname:   strings.Join([]string{strings.Repeat("a", 63), strings.Repeat("b", 63), strings.Repeat("c", 63), strings.Repeat("d", 63)}, "."),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeQName(tt.qname)
			if (err != nil) != tt.wantErr {
				t.Fatalf("encodeQName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("encodeQName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildResponseDoesNotSetRA(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	response := buildTestResponse(t, query)

	flags := binary.BigEndian.Uint16(response[2:4])

	if flags&0x0080 != 0 {
		t.Fatalf("RA flag is set, want RA=false; flags=%016b", flags)
	}
}

func TestBuildResponseSetsQR(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	response := buildTestResponse(t, query)

	flags := binary.BigEndian.Uint16(response[2:4])

	if flags&0x8000 == 0 {
		t.Fatalf("QR flag is not set; flags=%016b", flags)
	}
}

func TestBuildResponseCopiesRD(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	response := buildTestResponse(t, query)

	flags := binary.BigEndian.Uint16(response[2:4])

	if flags&0x0100 == 0 {
		t.Fatalf("RD flag was not copied; flags=%016b", flags)
	}
}

func TestBuildResponseReturnsAAnswerForTypeAClassIN(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", ancount)
	}

	questionEnd := len(query)
	if !bytes.Equal(response[HeaderSize:questionEnd], query[HeaderSize:]) {
		t.Fatalf("response question = %v, want %v", response[HeaderSize:questionEnd], query[HeaderSize:])
	}

	answer := response[questionEnd:]

	want := []byte{
		0xc0, 0x0c, // NAME pointer
		0x00, 0x01, // TYPE = A
		0x00, 0x01, // CLASS = IN
		0x00, 0x00, 0x00, 0x3c, // TTL = 60
		0x00, 0x04, // RDLENGTH = 4
		1, 2, 3, 4, // RDATA
	}

	if !bytes.Equal(answer, want) {
		t.Fatalf("answer = %v, want %v", answer, want)
	}
}

func TestBuildResponseMatchesConfiguredNameCaseInsensitively(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	copy(query[13:20], "ExAmPlE")
	copy(query[21:24], "CoM")

	response := buildTestResponse(t, query)

	anCount := binary.BigEndian.Uint16(response[6:8])
	if anCount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", anCount)
	}

	if !bytes.Equal(response[len(response)-4:], []byte{1, 2, 3, 4}) {
		t.Fatalf("RDATA = %v, want [1 2 3 4]", response[len(response)-4:])
	}
}

func TestBuildResponseReturnsConfiguredTestLocalRecord(t *testing.T) {
	query := sampleTestLocalAQuery()
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", ancount)
	}

	questionEnd := len(query)
	if !bytes.Equal(response[HeaderSize:questionEnd], query[HeaderSize:]) {
		t.Fatalf("response question = %v, want %v", response[HeaderSize:questionEnd], query[HeaderSize:])
	}

	answer := response[questionEnd:]
	want := []byte{
		0xc0, 0x0c, // NAME pointer
		0x00, 0x01, // TYPE = A
		0x00, 0x01, // CLASS = IN
		0x00, 0x00, 0x00, 0x3c, // TTL = 60
		0x00, 0x04, // RDLENGTH = 4
		5, 6, 7, 8, // RDATA
	}

	if !bytes.Equal(answer, want) {
		t.Fatalf("answer = %v, want %v", answer, want)
	}
}

func TestBuildResponseReturnsNXDOMAINForUnknownDomain(t *testing.T) {
	query := sampleOtherDomainAQuery()
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", ancount)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	rcode := flags & 0x000f
	if rcode != rCodeNXDomain {
		t.Fatalf("RCODE = %d, want %d (NXDOMAIN)", rcode, rCodeNXDomain)
	}

	if !bytes.Equal(response[HeaderSize:], query[HeaderSize:]) {
		t.Fatalf("response question = %v, want %v", response[HeaderSize:], query[HeaderSize:])
	}
}

func TestBuildResponseNoAnswerForUnsupportedType(t *testing.T) {
	const TypeAAAA uint16 = 28

	query := sampleQueryWithTypeClass(TypeAAAA, ClassIN)
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", ancount)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	rcode := flags & 0x000f
	if rcode != 0 {
		t.Fatalf("RCODE = %d, want 0 (NOERROR)", rcode)
	}

	if len(response) != len(query) {
		t.Fatalf("response length = %d, want %d", len(response), len(query))
	}
}

func TestBuildResponseNoAnswerForUnsupportedClass(t *testing.T) {
	const ClassCH uint16 = 3

	query := sampleQueryWithTypeClass(TypeA, ClassCH)
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", ancount)
	}

	if len(response) != len(query) {
		t.Fatalf("response length = %d, want %d", len(response), len(query))
	}
}
