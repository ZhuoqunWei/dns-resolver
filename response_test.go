package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

const TypeTXT uint16 = 16

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

func sampleMissingSubdomainAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: missing.example.com
		0x07, 'm', 'i', 's', 's', 'i', 'n', 'g',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE: A, QCLASS: IN
		0x00, 0x01,
		0x00, 0x01,
	}
}

func sampleTestExampleAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: test.example.com
		0x04, 't', 'e', 's', 't',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE: A, QCLASS: IN
		0x00, 0x01,
		0x00, 0x01,
	}
}

func samplePoolExampleAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD = true
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0

		// QNAME: pool.example.com
		0x04, 'p', 'o', 'o', 'l',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE: A, QCLASS: IN
		0x00, 0x01,
		0x00, 0x01,
	}
}

func sampleEDNSQuery(query []byte, udpPayloadSize uint16) []byte {
	return sampleEDNSQueryWithMetadata(query, udpPayloadSize, 0, nil)
}

func sampleEDNSQueryWithMetadata(
	query []byte,
	udpPayloadSize uint16,
	ttl uint32,
	options []byte,
) []byte {
	result := append([]byte(nil), query...)
	binary.BigEndian.PutUint16(result[10:12], 1)

	opt := make([]byte, 11+len(options))
	opt[0] = 0x00
	binary.BigEndian.PutUint16(opt[1:3], TypeOPT)
	binary.BigEndian.PutUint16(opt[3:5], udpPayloadSize)
	binary.BigEndian.PutUint32(opt[5:9], ttl)
	binary.BigEndian.PutUint16(opt[9:11], uint16(len(options)))
	copy(opt[11:], options)

	return append(result, opt...)
}

func buildTestResponse(t *testing.T, query []byte) []byte {
	t.Helper()

	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	response, err := buildResponse(msg, testZone())
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}

	return response
}

type testResourceRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	RData []byte
	Next  int
}

func parseTestResourceRecord(t *testing.T, message []byte, offset int) testResourceRecord {
	t.Helper()

	if offset >= len(message) {
		t.Fatalf("resource record offset %d exceeds message length %d", offset, len(message))
	}

	var name string
	if message[offset]&0xc0 == 0xc0 {
		if offset+2 > len(message) {
			t.Fatal("truncated resource record name pointer")
		}
		pointer := int(binary.BigEndian.Uint16(message[offset:offset+2]) & 0x3fff)
		var err error
		name, _, err = parseQName(message, pointer)
		if err != nil {
			t.Fatalf("parse resource record name pointer: %v", err)
		}
		offset += 2
	} else {
		var err error
		name, offset, err = parseQName(message, offset)
		if err != nil {
			t.Fatalf("parse resource record name: %v", err)
		}
	}

	if offset+10 > len(message) {
		t.Fatal("truncated resource record fields")
	}
	recordType := binary.BigEndian.Uint16(message[offset : offset+2])
	class := binary.BigEndian.Uint16(message[offset+2 : offset+4])
	ttl := binary.BigEndian.Uint32(message[offset+4 : offset+8])
	rDataLength := int(binary.BigEndian.Uint16(message[offset+8 : offset+10]))
	rDataStart := offset + 10
	rDataEnd := rDataStart + rDataLength
	if rDataEnd > len(message) {
		t.Fatal("truncated resource record RDATA")
	}

	return testResourceRecord{
		Name:  name,
		Type:  recordType,
		Class: class,
		TTL:   ttl,
		RData: message[rDataStart:rDataEnd],
		Next:  rDataEnd,
	}
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
	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0400 == 0 {
		t.Fatalf("AA flag is not set for in-zone answer; flags=%016b", flags)
	}
	if flags&0x0200 != 0 {
		t.Fatalf("TC flag is set for a response that fits; flags=%016b", flags)
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

func TestBuildResponseReturnsAAAAAnswer(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeAAAA, ClassIN)
	response := buildTestResponse(t, query)

	if anCount := binary.BigEndian.Uint16(response[6:8]); anCount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", anCount)
	}

	answer := parseTestResourceRecord(t, response, len(query))
	if answer.Name != "example.com" {
		t.Fatalf("answer name = %q, want %q", answer.Name, "example.com")
	}
	if answer.Type != TypeAAAA {
		t.Fatalf("answer TYPE = %d, want %d", answer.Type, TypeAAAA)
	}
	if answer.Class != ClassIN {
		t.Fatalf("answer CLASS = %d, want %d", answer.Class, ClassIN)
	}
	if answer.TTL != 120 {
		t.Fatalf("answer TTL = %d, want 120", answer.TTL)
	}
	wantRData := []byte{
		0x20, 0x01, 0x0d, 0xb8,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
	}
	if !bytes.Equal(answer.RData, wantRData) {
		t.Fatalf("answer RDATA = %v, want %v", answer.RData, wantRData)
	}
}

func TestBuildResponseReturnsSOAAnswer(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeSOA, ClassIN)
	response := buildTestResponse(t, query)

	if anCount := binary.BigEndian.Uint16(response[6:8]); anCount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", anCount)
	}
	if nsCount := binary.BigEndian.Uint16(response[8:10]); nsCount != 0 {
		t.Fatalf("NSCOUNT = %d, want 0", nsCount)
	}

	answer := parseTestResourceRecord(t, response, len(query))
	if answer.Type != TypeSOA {
		t.Fatalf("answer TYPE = %d, want %d", answer.Type, TypeSOA)
	}
	if answer.TTL != 300 {
		t.Fatalf("answer TTL = %d, want 300", answer.TTL)
	}
	if !bytes.Equal(answer.RData, testSOARData()) {
		t.Fatalf("answer RDATA = %v, want %v", answer.RData, testSOARData())
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

func TestBuildResponseUsesConfiguredTTL(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	zone := Zone{
		Origin: "example.com",
		Records: map[string]map[uint16][]Record{
			"example.com": {
				TypeA: {
					{
						TTL:   300,
						RData: []byte{1, 2, 3, 4},
					},
				},
			},
		},
	}

	response, err := buildResponse(msg, zone)
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}

	answer := response[len(query):]
	ttl := binary.BigEndian.Uint32(answer[6:10])
	if ttl != 300 {
		t.Fatalf("TTL = %d, want 300", ttl)
	}
}

func TestBuildResponseReturnsConfiguredTestExampleRecord(t *testing.T) {
	query := sampleTestExampleAQuery()
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
		0x00, 0x00, 0x01, 0x2c, // TTL = 300
		0x00, 0x04, // RDLENGTH = 4
		5, 6, 7, 8, // RDATA
	}

	if !bytes.Equal(answer, want) {
		t.Fatalf("answer = %v, want %v", answer, want)
	}
}

func TestBuildResponseReturnsAllRecordsInRRset(t *testing.T) {
	query := samplePoolExampleAQuery()
	response := buildTestResponse(t, query)

	if anCount := binary.BigEndian.Uint16(response[6:8]); anCount != 2 {
		t.Fatalf("ANCOUNT = %d, want 2", anCount)
	}

	wantRData := [][]byte{
		{192, 0, 2, 10},
		{192, 0, 2, 11},
	}
	offset := len(query)
	for i, want := range wantRData {
		answer := parseTestResourceRecord(t, response, offset)
		if answer.Name != "pool.example.com" {
			t.Fatalf("answer %d name = %q, want %q", i, answer.Name, "pool.example.com")
		}
		if answer.Type != TypeA {
			t.Fatalf("answer %d TYPE = %d, want %d", i, answer.Type, TypeA)
		}
		if answer.Class != ClassIN {
			t.Fatalf("answer %d CLASS = %d, want %d", i, answer.Class, ClassIN)
		}
		if answer.TTL != 90 {
			t.Fatalf("answer %d TTL = %d, want 90", i, answer.TTL)
		}
		if !bytes.Equal(answer.RData, want) {
			t.Fatalf("answer %d RDATA = %v, want %v", i, answer.RData, want)
		}
		offset = answer.Next
	}
	if offset != len(response) {
		t.Fatalf("parsed response through offset %d, response length is %d", offset, len(response))
	}
}

func TestBuildResponseIncludesEDNSOPT(t *testing.T) {
	baseQuery := sampleQueryWithTypeClass(TypeA, ClassIN)
	query := sampleEDNSQuery(baseQuery, 1232)
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	response, err := buildResponse(msg, testZone())
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}
	if additionalCount := binary.BigEndian.Uint16(response[10:12]); additionalCount != 1 {
		t.Fatalf("ARCOUNT = %d, want 1", additionalCount)
	}

	answer := parseTestResourceRecord(t, response, len(baseQuery))
	opt := parseTestResourceRecord(t, response, answer.Next)
	if opt.Name != "" {
		t.Fatalf("OPT owner name = %q, want root", opt.Name)
	}
	if opt.Type != TypeOPT {
		t.Fatalf("OPT TYPE = %d, want %d", opt.Type, TypeOPT)
	}
	if opt.Class != maxEDNSUDPMessageSize {
		t.Fatalf("OPT UDP payload size = %d, want %d", opt.Class, maxEDNSUDPMessageSize)
	}
	if opt.TTL != 0 {
		t.Fatalf("OPT TTL metadata = %d, want 0", opt.TTL)
	}
	if len(opt.RData) != 0 {
		t.Fatalf("OPT RDATA length = %d, want 0", len(opt.RData))
	}
	if opt.Next != len(response) {
		t.Fatalf("parsed response through offset %d, response length is %d", opt.Next, len(response))
	}
}

func TestBuildResponseReturnsBADVERSForUnsupportedEDNSVersion(t *testing.T) {
	baseQuery := sampleQueryWithTypeClass(TypeA, ClassIN)
	query := sampleEDNSQueryWithMetadata(baseQuery, 1232, 0x00010000, nil)
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	response, err := buildResponse(msg, testZone())
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}
	if answerCount := binary.BigEndian.Uint16(response[6:8]); answerCount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", answerCount)
	}
	if additionalCount := binary.BigEndian.Uint16(response[10:12]); additionalCount != 1 {
		t.Fatalf("ARCOUNT = %d, want 1", additionalCount)
	}
	if headerRCode := binary.BigEndian.Uint16(response[2:4]) & 0x000f; headerRCode != 0 {
		t.Fatalf("header RCODE = %d, want 0 as lower BADVERS bits", headerRCode)
	}

	opt := parseTestResourceRecord(t, response, len(baseQuery))
	if extendedRCode := uint8(opt.TTL >> 24); extendedRCode != extendedRCodeBadVersion {
		t.Fatalf("extended RCODE = %d, want %d (BADVERS)", extendedRCode, extendedRCodeBadVersion)
	}
	if version := uint8(opt.TTL >> 16); version != 0 {
		t.Fatalf("response EDNS version = %d, want 0", version)
	}
}

func TestUDPResponseSizeLimit(t *testing.T) {
	tests := []struct {
		name      string
		edns      *EDNS
		wantLimit int
	}{
		{name: "classic DNS", wantLimit: maxDNSUDPMessageSize},
		{name: "advertised value below 512", edns: &EDNS{UDPSize: 128}, wantLimit: maxDNSUDPMessageSize},
		{name: "advertised value within server cap", edns: &EDNS{UDPSize: 900}, wantLimit: 900},
		{name: "advertised value above server cap", edns: &EDNS{UDPSize: 4096}, wantLimit: maxEDNSUDPMessageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := udpResponseSizeLimit(Message{EDNS: tt.edns})
			if got != tt.wantLimit {
				t.Fatalf("udpResponseSizeLimit() = %d, want %d", got, tt.wantLimit)
			}
		})
	}
}

func TestHandlePacketUsesNegotiatedEDNSSize(t *testing.T) {
	tests := []struct {
		name            string
		advertisedSize  uint16
		recordCount     int
		wantAnswerCount uint16
		wantTruncated   bool
		wantMaxSize     int
	}{
		{
			name:            "client size fits complete RRset",
			advertisedSize:  1232,
			recordCount:     40,
			wantAnswerCount: 40,
			wantMaxSize:     1232,
		},
		{
			name:            "client size limits response",
			advertisedSize:  600,
			recordCount:     40,
			wantAnswerCount: 34,
			wantTruncated:   true,
			wantMaxSize:     600,
		},
		{
			name:            "server cap limits response",
			advertisedSize:  4096,
			recordCount:     100,
			wantAnswerCount: 74,
			wantTruncated:   true,
			wantMaxSize:     maxEDNSUDPMessageSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseQuery := samplePoolExampleAQuery()
			query := sampleEDNSQuery(baseQuery, tt.advertisedSize)
			_, response, err := handlePacket(query, testZoneWithLargeARRset(tt.recordCount))
			if err != nil {
				t.Fatalf("handlePacket returned error: %v", err)
			}

			if len(response) > tt.wantMaxSize {
				t.Fatalf("response length = %d, want at most %d", len(response), tt.wantMaxSize)
			}
			if answerCount := binary.BigEndian.Uint16(response[6:8]); answerCount != tt.wantAnswerCount {
				t.Fatalf("ANCOUNT = %d, want %d", answerCount, tt.wantAnswerCount)
			}
			if additionalCount := binary.BigEndian.Uint16(response[10:12]); additionalCount != 1 {
				t.Fatalf("ARCOUNT = %d, want 1", additionalCount)
			}
			flags := binary.BigEndian.Uint16(response[2:4])
			if truncated := flags&0x0200 != 0; truncated != tt.wantTruncated {
				t.Fatalf("TC = %t, want %t; flags=%016b", truncated, tt.wantTruncated, flags)
			}
		})
	}
}

func TestBuildResponseTruncatesOversizedRRsetAtRecordBoundary(t *testing.T) {
	query := samplePoolExampleAQuery()
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	response, err := buildResponse(msg, testZoneWithLargeARRset(40))
	if err != nil {
		t.Fatalf("buildResponse returned error: %v", err)
	}

	if len(response) > maxDNSUDPMessageSize {
		t.Fatalf("response length = %d, want at most %d", len(response), maxDNSUDPMessageSize)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0200 == 0 {
		t.Fatalf("TC flag is not set for truncated response; flags=%016b", flags)
	}

	const wantAnswerCount uint16 = 29
	answerCount := binary.BigEndian.Uint16(response[6:8])
	if answerCount != wantAnswerCount {
		t.Fatalf("ANCOUNT = %d, want %d", answerCount, wantAnswerCount)
	}
	if authorityCount := binary.BigEndian.Uint16(response[8:10]); authorityCount != 0 {
		t.Fatalf("NSCOUNT = %d, want 0", authorityCount)
	}

	offset := len(query)
	for i := 0; i < int(answerCount); i++ {
		answer := parseTestResourceRecord(t, response, offset)
		wantRData := []byte{192, 0, 2, byte(i + 1)}
		if !bytes.Equal(answer.RData, wantRData) {
			t.Fatalf("answer %d RDATA = %v, want %v", i, answer.RData, wantRData)
		}
		offset = answer.Next
	}
	if offset != len(response) {
		t.Fatalf("parsed response through offset %d, response length is %d", offset, len(response))
	}
}

func TestBuildResponseTruncatesAuthorityAtRecordBoundary(t *testing.T) {
	query := sampleMissingSubdomainAQuery()
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	response, err := buildResponseWithLimit(msg, testZone(), len(query)+1)
	if err != nil {
		t.Fatalf("buildResponseWithLimit returned error: %v", err)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0200 == 0 {
		t.Fatalf("TC flag is not set when authority record is omitted; flags=%016b", flags)
	}
	if rCode := flags & 0x000f; rCode != rCodeNXDomain {
		t.Fatalf("RCODE = %d, want %d (NXDOMAIN)", rCode, rCodeNXDomain)
	}
	if answerCount := binary.BigEndian.Uint16(response[6:8]); answerCount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", answerCount)
	}
	if authorityCount := binary.BigEndian.Uint16(response[8:10]); authorityCount != 0 {
		t.Fatalf("NSCOUNT = %d, want 0", authorityCount)
	}
	if len(response) != len(query) {
		t.Fatalf("response length = %d, want %d", len(response), len(query))
	}
}

func TestBuildResponseRejectsLimitSmallerThanQuestion(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	msg, err := parseMessage(query)
	if err != nil {
		t.Fatalf("parseMessage returned error: %v", err)
	}

	if _, err := buildResponseWithLimit(msg, testZone(), len(query)-1); err == nil {
		t.Fatal("buildResponseWithLimit returned nil error for a limit smaller than the header and question")
	}
}

func TestBuildResponseReturnsNXDOMAINForMissingInZoneName(t *testing.T) {
	query := sampleMissingSubdomainAQuery()
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", ancount)
	}
	nsCount := binary.BigEndian.Uint16(response[8:10])
	if nsCount != 1 {
		t.Fatalf("NSCOUNT = %d, want 1", nsCount)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	rcode := flags & 0x000f
	if rcode != rCodeNXDomain {
		t.Fatalf("RCODE = %d, want %d (NXDOMAIN)", rcode, rCodeNXDomain)
	}
	if flags&0x0400 == 0 {
		t.Fatalf("AA flag is not set for in-zone NXDOMAIN; flags=%016b", flags)
	}

	if !bytes.Equal(response[HeaderSize:len(query)], query[HeaderSize:]) {
		t.Fatalf("response question = %v, want %v", response[HeaderSize:len(query)], query[HeaderSize:])
	}

	authority := parseTestResourceRecord(t, response, len(query))
	if authority.Name != "example.com" {
		t.Fatalf("authority name = %q, want %q", authority.Name, "example.com")
	}
	if authority.Type != TypeSOA {
		t.Fatalf("authority TYPE = %d, want %d", authority.Type, TypeSOA)
	}
	if authority.TTL != 120 {
		t.Fatalf("negative SOA TTL = %d, want 120", authority.TTL)
	}
	if !bytes.Equal(authority.RData, testSOARData()) {
		t.Fatalf("authority RDATA = %v, want %v", authority.RData, testSOARData())
	}
}

func TestBuildResponseReturnsRefusedForOutOfZoneName(t *testing.T) {
	query := sampleOtherDomainAQuery()
	response := buildTestResponse(t, query)

	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Fatalf("ANCOUNT = %d, want 0", ancount)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	rcode := flags & 0x000f
	if rcode != rCodeRefused {
		t.Fatalf("RCODE = %d, want %d (REFUSED)", rcode, rCodeRefused)
	}
	if flags&0x0400 != 0 {
		t.Fatalf("AA flag is set for refused out-of-zone query; flags=%016b", flags)
	}
	if nsCount := binary.BigEndian.Uint16(response[8:10]); nsCount != 0 {
		t.Fatalf("NSCOUNT = %d, want 0", nsCount)
	}

	if !bytes.Equal(response[HeaderSize:], query[HeaderSize:]) {
		t.Fatalf("response question = %v, want %v", response[HeaderSize:], query[HeaderSize:])
	}
}

func TestBuildResponseNoAnswerForUnsupportedType(t *testing.T) {
	query := sampleQueryWithTypeClass(TypeTXT, ClassIN)
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
	if flags&0x0400 == 0 {
		t.Fatalf("AA flag is not set for in-zone NODATA; flags=%016b", flags)
	}
	if nsCount := binary.BigEndian.Uint16(response[8:10]); nsCount != 1 {
		t.Fatalf("NSCOUNT = %d, want 1", nsCount)
	}

	authority := parseTestResourceRecord(t, response, len(query))
	if authority.Type != TypeSOA {
		t.Fatalf("authority TYPE = %d, want %d", authority.Type, TypeSOA)
	}
	if authority.TTL != 120 {
		t.Fatalf("negative SOA TTL = %d, want 120", authority.TTL)
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
	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0400 != 0 {
		t.Fatalf("AA flag is set for unsupported class; flags=%016b", flags)
	}
	if nsCount := binary.BigEndian.Uint16(response[8:10]); nsCount != 0 {
		t.Fatalf("NSCOUNT = %d, want 0", nsCount)
	}

	if len(response) != len(query) {
		t.Fatalf("response length = %d, want %d", len(response), len(query))
	}
}

func TestAppendResourceRecordRejectsOversizedRData(t *testing.T) {
	_, err := appendResourceRecord(nil, responseRecord{
		Name: "example.com",
		Type: TypeA,
		Record: Record{
			RData: make([]byte, 0x10000),
		},
	})
	if err == nil {
		t.Fatal("appendResourceRecord returned nil error for oversized RDATA")
	}
}
