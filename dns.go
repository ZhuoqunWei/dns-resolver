package main

import (
	"fmt"
	"strings"
)

func readU16(data []byte, offset int) (uint16, error) {
	// Check if the offset is valid
	if offset < 0 || offset >= len(data) {
		return 0, fmt.Errorf("offset %d is out of bounds for data length %d", offset, len(data))
	}
	// Check if there are enough bytes to read
	if len(data) < offset+2 {
		return 0, fmt.Errorf("not enough bytes to read uint16 at offset %d", offset)
	}

	// Read two bytes and convert to uint16
	// The first byte is the high byte, and the second byte is the low byte
	return uint16(data[offset])<<8 | uint16(data[offset+1]), nil
}

type Header struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

// parse 12 bytes dns header
func parseHeader(data []byte) (Header, error) {
	if len(data) < 12 {
		return Header{}, fmt.Errorf("data too short to contain DNS header")
	}

	id, err := readU16(data, 0)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read ID: %v", err)
	}
	flags, err := readU16(data, 2)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read Flags: %v", err)
	}
	qdCount, err := readU16(data, 4)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read QDCount: %v", err)
	}
	anCount, err := readU16(data, 6)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read ANCount: %v", err)
	}
	nsCount, err := readU16(data, 8)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read NSCount: %v", err)
	}
	arCount, err := readU16(data, 10)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read ARCount: %v", err)
	}

	return Header{
		ID:      id,
		Flags:   flags,
		QDCount: qdCount,
		ANCount: anCount,
		NSCount: nsCount,
		ARCount: arCount,
	}, nil
}

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

func parseFlags(flags uint16) Flags {
	// Extract individual fields from the flags
	return Flags{
		QR:     (flags & 0x8000) != 0,
		Opcode: uint8((flags >> 11) & 0xF),
		AA:     (flags & 0x0400) != 0,
		TC:     (flags & 0x0200) != 0,
		RD:     (flags & 0x0100) != 0,
		RA:     (flags & 0x0080) != 0,
		Z:      uint8((flags >> 4) & 0x7),
		RCode:  uint8((flags >> 0) & 0xF),
	}
}

func parseQName(data []byte, offset int) (string, int, error) {
	// first check the offset is valid
	if offset < 0 || offset >= len(data) {
		return "", -1, fmt.Errorf("offset %d is out of bounds for data length %d", offset, len(data))
	}

	// create label slice
	labels := []string{}

	// Moving cursor
	i := offset

	// loop through labels
	for {
		// check if out of bounds
		if i >= len(data) {
			return "", -1, fmt.Errorf("offset %d is out of bounds for data length %d", i, len(data))
		}

		length := int(data[i])
		i++

		if length == 0 {
			return strings.Join(labels, "."), i, nil
		}
		if length > 63 {
			return "", -1, fmt.Errorf("label length %d exceeds maximum of 63", length)
		}
		if i+length > len(data) {
			return "", -1, fmt.Errorf("label length %d exceeds remaining data length %d", length, len(data)-i)
		}

		label := string(data[i : i+length])
		labels = append(labels, label)
		i += length
	}
}

type Question struct {
	Name   string
	QType  uint16
	QClass uint16
}

func parseQuestion(data []byte, offset int) (Question, int, error) {

	name, offset, err := parseQName(data, offset)
	if err != nil {
		return Question{}, -1, fmt.Errorf("error parsing qname: %w", err)
	}
	// read data by offset
	qtype, err := readU16(data, offset)
	if err != nil {
		return Question{}, -1, fmt.Errorf("error parsing qtype: %w", err)
	}
	offset += 2
	qclass, err := readU16(data, offset)
	if err != nil {
		return Question{}, -1, fmt.Errorf("error parsing qclass: %w", err)
	}
	offset += 2

	return Question{
		Name:   name,
		QType:  qtype,
		QClass: qclass,
	}, offset, nil

}

type Message struct {
	Header   Header
	Flags    Flags
	Question Question
}

func parseMessage(data []byte) (Message, error) {
	header, err := parseHeader(data)
	if err != nil {
		return Message{}, fmt.Errorf("parse header: %w", err)
	}

	if header.QDCount != 1 {
		return Message{}, fmt.Errorf("expected exactly 1 question, got %d", header.QDCount)
	}

	flags := parseFlags(header.Flags)

	question, _, err := parseQuestion(data, 12)
	if err != nil {
		return Message{}, fmt.Errorf("parse question: %w", err)
	}

	return Message{
		Header:   header,
		Flags:    flags,
		Question: question,
	}, nil
}