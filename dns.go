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
