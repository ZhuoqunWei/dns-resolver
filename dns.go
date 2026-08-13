package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	HeaderSize = 12

	TypeA    uint16 = 1
	TypeSOA  uint16 = 6
	TypeAAAA uint16 = 28
	TypeOPT  uint16 = 41
	ClassIN  uint16 = 1
)

func readU16(data []byte, offset int) (uint16, error) {
	if offset < 0 || offset+2 > len(data) {
		return 0, fmt.Errorf("not enough bytes to read uint16 at offset %d", offset)
	}

	return uint16(data[offset])<<8 | uint16(data[offset+1]), nil
}

func readU32(data []byte, offset int) (uint32, error) {
	if offset < 0 || offset+4 > len(data) {
		return 0, fmt.Errorf("not enough bytes to read uint32 at offset %d", offset)
	}

	return binary.BigEndian.Uint32(data[offset : offset+4]), nil
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
	if len(data) < HeaderSize {
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

type EDNS struct {
	UDPSize       uint16
	ExtendedRCode uint8
	Version       uint8
	DNSSECOK      bool
}

type wireResourceRecord struct {
	Name   string
	Type   uint16
	Class  uint16
	TTL    uint32
	RData  []byte
	Offset int
}

func parseWireResourceRecord(data []byte, offset int) (wireResourceRecord, error) {
	name, offset, err := parseQName(data, offset)
	if err != nil {
		return wireResourceRecord{}, fmt.Errorf("parse owner name: %w", err)
	}

	recordType, err := readU16(data, offset)
	if err != nil {
		return wireResourceRecord{}, fmt.Errorf("parse type: %w", err)
	}
	offset += 2

	class, err := readU16(data, offset)
	if err != nil {
		return wireResourceRecord{}, fmt.Errorf("parse class: %w", err)
	}
	offset += 2

	ttl, err := readU32(data, offset)
	if err != nil {
		return wireResourceRecord{}, fmt.Errorf("parse TTL: %w", err)
	}
	offset += 4

	rDataLength, err := readU16(data, offset)
	if err != nil {
		return wireResourceRecord{}, fmt.Errorf("parse RDLENGTH: %w", err)
	}
	offset += 2

	rDataEnd := offset + int(rDataLength)
	if rDataEnd > len(data) {
		return wireResourceRecord{}, fmt.Errorf(
			"RDLENGTH %d exceeds remaining message length %d",
			rDataLength,
			len(data)-offset,
		)
	}

	return wireResourceRecord{
		Name:   name,
		Type:   recordType,
		Class:  class,
		TTL:    ttl,
		RData:  data[offset:rDataEnd],
		Offset: rDataEnd,
	}, nil
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
	EDNS     *EDNS
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

	question, offset, err := parseQuestion(data, HeaderSize)
	if err != nil {
		return Message{}, fmt.Errorf("parse question: %w", err)
	}

	for i := 0; i < int(header.ANCount)+int(header.NSCount); i++ {
		record, err := parseWireResourceRecord(data, offset)
		if err != nil {
			return Message{}, fmt.Errorf("parse answer or authority record %d: %w", i, err)
		}
		if record.Type == TypeOPT {
			return Message{}, fmt.Errorf("OPT record must appear in the additional section")
		}
		offset = record.Offset
	}

	var edns *EDNS
	for i := 0; i < int(header.ARCount); i++ {
		record, err := parseWireResourceRecord(data, offset)
		if err != nil {
			return Message{}, fmt.Errorf("parse additional record %d: %w", i, err)
		}
		offset = record.Offset

		if record.Type != TypeOPT {
			continue
		}
		if edns != nil {
			return Message{}, fmt.Errorf("multiple OPT records are not allowed")
		}
		if record.Name != "" {
			return Message{}, fmt.Errorf("OPT owner name must be the root domain")
		}

		edns = &EDNS{
			UDPSize:       record.Class,
			ExtendedRCode: uint8(record.TTL >> 24),
			Version:       uint8(record.TTL >> 16),
			DNSSECOK:      record.TTL&0x00008000 != 0,
		}
	}

	if offset != len(data) {
		return Message{}, fmt.Errorf("%d trailing bytes after declared DNS sections", len(data)-offset)
	}

	return Message{
		Header:   header,
		Flags:    flags,
		Question: question,
		EDNS:     edns,
	}, nil
}
