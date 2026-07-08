package main

import (
	"bytes"
	"testing"
)

func sampleAQuery() []byte {
	return []byte{
		// Header
		0x12, 0x34, // ID
		0x01, 0x00, // Flags
		0x00, 0x01, // QDCOUNT
		0x00, 0x00, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT

		// QNAME: example.com
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,

		// QTYPE + QCLASS
		0x00, 0x01, // A
		0x00, 0x01, // IN
	}
}

func TestBuildAResponseCopiesTransactionID(t *testing.T) {
	query := sampleAQuery()

	response, err := buildAResponse(query)
	if err != nil {
		t.Fatalf("buildAResponse returned error: %v", err)
	}

	if response[0] != 0x12 || response[1] != 0x34 {
		t.Fatalf("ID = %02x %02x, want 12 34", response[0], response[1])
	}
}

func TestBuildAResponseSetsCounts(t *testing.T) {
	query := sampleAQuery()

	response, err := buildAResponse(query)
	if err != nil {
		t.Fatalf("buildAResponse returned error: %v", err)
	}

	want := []byte{
		0x00, 0x01, // QDCOUNT = 1
		0x00, 0x01, // ANCOUNT = 1
		0x00, 0x00, // NSCOUNT = 0
		0x00, 0x00, // ARCOUNT = 0
	}

	got := response[4:12]

	if !bytes.Equal(got, want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
}

func TestBuildAResponseCopiesQuestionSection(t *testing.T) {
	query := sampleAQuery()

	response, err := buildAResponse(query)
	if err != nil {
		t.Fatalf("buildAResponse returned error: %v", err)
	}

	questionEnd, err := findQuestionEnd(query)
	if err != nil {
		t.Fatalf("findQuestionEnd returned error: %v", err)
	}

	gotQuestion := response[12:questionEnd]
	wantQuestion := query[12:questionEnd]

	if !bytes.Equal(gotQuestion, wantQuestion) {
		t.Fatalf("question section = %v, want %v", gotQuestion, wantQuestion)
	}
}

func TestBuildAResponseAppendsAnswerRecord(t *testing.T) {
	query := sampleAQuery()

	response, err := buildAResponse(query)
	if err != nil {
		t.Fatalf("buildAResponse returned error: %v", err)
	}

	questionEnd, err := findQuestionEnd(query)
	if err != nil {
		t.Fatalf("findQuestionEnd returned error: %v", err)
	}

	answer := response[questionEnd:]

	want := []byte{
		0xc0, 0x0c, // NAME pointer to QNAME
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

func TestBuildAResponseLength(t *testing.T) {
	query := sampleAQuery()

	response, err := buildAResponse(query)
	if err != nil {
		t.Fatalf("buildAResponse returned error: %v", err)
	}

	// Answer record length:
	// NAME 2 + TYPE 2 + CLASS 2 + TTL 4 + RDLENGTH 2 + RDATA 4 = 16
	wantLen := len(query) + 16

	if len(response) != wantLen {
		t.Fatalf("response length = %d, want %d", len(response), wantLen)
	}
}