package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const rCodeNXDomain uint16 = 3

func buildResponse(msg Message, records map[string][4]byte) ([]byte, error) {
	question := msg.Question
	rData, exists := records[question.Name]

	hasAnswer := question.QType == TypeA &&
		question.QClass == ClassIN &&
		exists

	encodedName, err := encodeQName(question.Name)
	if err != nil {
		return nil, fmt.Errorf("encode qname: %w", err)
	}

	response := make([]byte, 0)

	// ID: copy from parsed query message
	idBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(idBytes, msg.Header.ID)
	response = append(response, idBytes...)

	// Flags:
	// QR = 1 response
	// RD = copied from query
	// RA = 0 because we do not support recursion yet
	// RCODE = NXDOMAIN when the queried name is not configured
	var flags uint16 = 0x8000 // QR = 1

	if msg.Flags.RD {
		flags |= 0x0100 // copy RD
	}
	if !exists {
		flags |= rCodeNXDomain
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
	response = append(response, encodedName...)
	questionFields := make([]byte, 4)
	binary.BigEndian.PutUint16(questionFields[0:2], question.QType)
	binary.BigEndian.PutUint16(questionFields[2:4], question.QClass)
	response = append(response, questionFields...)

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

	// RDATA = configured IPv4 address
	response = append(response, rData[:]...)

	return response, nil
}

func encodeQName(name string) ([]byte, error) {
	if name == "" {
		return []byte{0}, nil
	}

	encoded := make([]byte, 0, len(name)+2)
	for _, label := range strings.Split(name, ".") {
		if label == "" {
			return nil, fmt.Errorf("empty label in name %q", name)
		}
		if len(label) > 63 {
			return nil, fmt.Errorf("label %q exceeds 63 bytes", label)
		}

		encoded = append(encoded, byte(len(label)))
		encoded = append(encoded, label...)
	}

	encoded = append(encoded, 0)
	if len(encoded) > 255 {
		return nil, fmt.Errorf("encoded name %q exceeds 255 bytes", name)
	}

	return encoded, nil
}
