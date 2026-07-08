package main

import (
	"encoding/binary"
	"fmt"
)

func buildResponse(query []byte) ([]byte, error) {
	if len(query) < 12 {
		return nil, fmt.Errorf("query too short")
	}

	questionEnd, err := findQuestionEnd(query)
	if err != nil {
		return nil, err
	}

	qtype := binary.BigEndian.Uint16(query[questionEnd-4 : questionEnd-2])
	qclass := binary.BigEndian.Uint16(query[questionEnd-2 : questionEnd])

	hasAnswer := qtype == TypeA && qclass == ClassIN

	response := make([]byte, 0)

	// ID: copy from query
	response = append(response, query[0], query[1])

	// Flags:
	// QR = 1 response
	// RD = copied from query
	// RA = 0 because we do not support recursion yet
	// RCODE = 0
	var flags uint16 = 0x8000 // QR = 1

	queryFlags := binary.BigEndian.Uint16(query[2:4])
	if queryFlags&0x0100 != 0 {
		flags |= 0x0100 // copy RD
	}

	flagsBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(flagsBytes, flags)
	response = append(response, flagsBytes...)

	// QDCOUNT = 1
	response = append(response, 0x00, 0x01)

	if hasAnswer {
		// ANCOUNT = 1
		response = append(response, 0x00, 0x01)
	} else {
		// ANCOUNT = 0
		response = append(response, 0x00, 0x00)
	}

	// NSCOUNT = 0
	response = append(response, 0x00, 0x00)

	// ARCOUNT = 0
	response = append(response, 0x00, 0x00)

	// Question section
	response = append(response, query[12:questionEnd]...)

	if !hasAnswer {
		return response, nil
	}

	// Answer section

	// NAME: pointer to QNAME at byte offset 12
	response = append(response, 0xc0, 0x0c)

	// TYPE = A
	response = append(response, 0x00, 0x01)

	// CLASS = IN
	response = append(response, 0x00, 0x01)

	// TTL = 60
	ttl := make([]byte, 4)
	binary.BigEndian.PutUint32(ttl, 60)
	response = append(response, ttl...)

	// RDLENGTH = 4
	response = append(response, 0x00, 0x04)

	// RDATA = 1.2.3.4
	response = append(response, 1, 2, 3, 4)

	return response, nil
}

func findQuestionEnd(query []byte) (int, error) {
	if len(query) < 12 {
		return 0, fmt.Errorf("query too short")
	}

	i := 12

	for {
		if i >= len(query) {
			return 0, fmt.Errorf("unterminated qname")
		}

		labelLen := int(query[i])
		i++

		if labelLen == 0 {
			break
		}

		if i+labelLen > len(query) {
			return 0, fmt.Errorf("label exceeds query length")
		}

		i += labelLen
	}

	if i+4 > len(query) {
		return 0, fmt.Errorf("missing qtype or qclass")
	}

	return i + 4, nil
}