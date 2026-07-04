package main

import "fmt"

func buildEmptyResponse(query []byte) ([]byte, error) {
	if len(query) < 12 {
		return nil, fmt.Errorf("query too short")
	}

	questionEnd, err := findQuestionEnd(query)
	if err != nil {
		return nil, err
	}

	response := make([]byte, 0)

	// Transaction ID: copy from query
	response = append(response, query[0], query[1])

	// Flags
	// 0x8180 means:
	// QR = 1 response
	// Opcode = 0 standard query
	// AA = 0
	// TC = 0
	// RD = 1
	// RA = 1
	// RCODE = 0 no error
	response = append(response, 0x81, 0x80)

	// QDCOUNT = 1
	response = append(response, 0x00, 0x01)

	// ANCOUNT = 0
	response = append(response, 0x00, 0x00)

	// NSCOUNT = 0
	response = append(response, 0x00, 0x00)

	// ARCOUNT = 0
	response = append(response, 0x00, 0x00)

	// Copy original question section:
	// QNAME + QTYPE + QCLASS
	response = append(response, query[12:questionEnd]...)

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