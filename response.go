package main

import (
	"encoding/binary"
	"fmt"
)


func buildAResponse(query []byte) ([]byte, error) {
	if len(query) < 12 {
		return nil, fmt.Errorf("query too short")
	}

	questionEnd, err := findQuestionEnd(query)
	if err != nil {
		return nil, err
	}

	response := make([]byte, 0)

	// Header

	// Transaction ID: copy from query
	response = append(response, query[0], query[1])

	// Flags: standard response, recursion available, no error
	// 0x8180:
	// QR = 1, RD = 1, RA = 1, RCODE = 0
	response = append(response, 0x81, 0x80)

	// QDCOUNT = 1
	response = append(response, 0x00, 0x01)

	// ANCOUNT = 1
	response = append(response, 0x00, 0x01)

	// NSCOUNT = 0
	response = append(response, 0x00, 0x00)

	// ARCOUNT = 0
	response = append(response, 0x00, 0x00)

	// Question section: copy QNAME + QTYPE + QCLASS
	response = append(response, query[12:questionEnd]...)

	// Answer section

	// NAME: pointer to byte offset 12, where QNAME starts
	// 0xc00c means "same name as the question"
	response = append(response, 0xc0, 0x0c)

	// TYPE = A
	response = append(response, 0x00, 0x01)

	// CLASS = IN
	response = append(response, 0x00, 0x01)

	// TTL = 60 seconds
	ttl := make([]byte, 4)
	binary.BigEndian.PutUint32(ttl, 60)
	response = append(response, ttl...)

	// RDLENGTH = 4 bytes for IPv4
	response = append(response, 0x00, 0x04)

	// RDATA = 1.2.3.4
	response = append(response, 1, 2, 3, 4)

	return response, nil
}

func findQuestionEnd(query []byte) (int, error) {
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

	// After QNAME, DNS question has:
	// QTYPE  = 2 bytes
	// QCLASS = 2 bytes
	if i+4 > len(query) {
		return 0, fmt.Errorf("missing qtype or qclass")
	}

	return i + 4, nil
}
