package main

import "fmt"

/*
*
1. readU16([]byte{0x12, 0x34}, 0) returns 4660
2. readU16([]byte{0x00, 0x01}, 0) returns 1
3. readU16([]byte{0x12}, 0) returns an error
4. go test ./... passes
5. commit made
*/
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
