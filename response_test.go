package main

import (
	"bytes"
	"testing"
)

func sampleQuery() []byte {
	return []byte{
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

		// QTYPE and QCLASS
		0x00, 0x01, // QTYPE = A
		0x00, 0x01, // QCLASS = IN
	}
}

func TestBuildEmptyResponseCopiesTransactionID(t *testing.T) {
	query := sampleQuery()

	response, err := buildEmptyResponse(query)
	if err != nil {
		t.Fatalf("buildEmptyResponse returned error: %v", err)
	}

	if response[0] != query[0] || response[1] != query[1] {
		t.Fatalf("response ID = %02x %02x, want %02x %02x",
			response[0], response[1], query[0], query[1])
	}
}

func TestBuildEmptyResponseSetsResponseFlags(t *testing.T) {
	query := sampleQuery()

	response, err := buildEmptyResponse(query)
	if err != nil {
		t.Fatalf("buildEmptyResponse returned error: %v", err)
	}

	if response[2] != 0x81 || response[3] != 0x80 {
		t.Fatalf("flags = %02x %02x, want 81 80", response[2], response[3])
	}
}

func TestBuildEmptyResponseSetsCounts(t *testing.T) {
	query := sampleQuery()

	response, err := buildEmptyResponse(query)
	if err != nil {
		t.Fatalf("buildEmptyResponse returned error: %v", err)
	}

	want := []byte{
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x00, // ANCOUNT = 0
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0
	}

	got := response[4:12]

	if !bytes.Equal(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
}

func TestBuildEmptyResponseCopiesQuestionSection(t *testing.T) {
	query := sampleQuery()

	response, err := buildEmptyResponse(query)
	if err != nil {
		t.Fatalf("buildEmptyResponse returned error: %v", err)
	}

	// Header is 12 bytes.
	// Everything after byte 12 should be the original question section.
	gotQuestion := response[12:]
	wantQuestion := query[12:]

	if !bytes.Equal(gotQuestion, wantQuestion) {
		t.Fatalf("question section = %v, want %v", gotQuestion, wantQuestion)
	}
}

func TestBuildEmptyResponseLength(t *testing.T) {
	query := sampleQuery()

	response, err := buildEmptyResponse(query)
	if err != nil {
		t.Fatalf("buildEmptyResponse returned error: %v", err)
	}

	// Empty response should contain:
	// 12-byte header + original question section.
	if len(response) != len(query) {
		t.Fatalf("response length = %d, want %d", len(response), len(query))
	}
}

func TestBuildEmptyResponseRejectsShortQuery(t *testing.T) {
	query := []byte{0x12, 0x34}

	_, err := buildEmptyResponse(query)
	if err == nil {
		t.Fatal("expected error for short query, got nil")
	}
}

func TestBuildEmptyResponseRejectsUnterminatedQName(t *testing.T) {
	query := []byte{
		// Header
		0x12, 0x34,
		0x01, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,

		// QNAME starts but never terminates with 0x00
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
	}

	_, err := buildEmptyResponse(query)
	if err == nil {
		t.Fatal("expected error for unterminated qname, got nil")
	}
}

func TestBuildEmptyResponseRejectsMissingQTypeQClass(t *testing.T) {
	query := []byte{
		// Header
		0x12, 0x34,
		0x01, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,

		// QNAME: example.com
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// Missing QTYPE and QCLASS
	}

	_, err := buildEmptyResponse(query)
	if err == nil {
		t.Fatal("expected error for missing qtype/qclass, got nil")
	}
}
