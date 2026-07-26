package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	rCodeNXDomain uint16 = 3
	rCodeRefused  uint16 = 5
)

func buildResponse(msg Message, zone Zone) ([]byte, error) {
	question := msg.Question
	name := canonicalName(question.Name)
	recordsByType, nameHasRecords := zone.Records[name]
	aRecords := recordsByType[TypeA]
	inZone := zone.contains(name)
	nameExists := zone.nameExists(name)

	hasAnswer := question.QType == TypeA &&
		question.QClass == ClassIN &&
		len(aRecords) > 0 &&
		nameHasRecords &&
		inZone

	var record Record
	if hasAnswer {
		record = aRecords[0]
	}

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
	// RCODE = NXDOMAIN for a missing in-zone name
	// RCODE = REFUSED for a name outside the configured zone
	var flags uint16 = 0x8000 // QR = 1

	if msg.Flags.RD {
		flags |= 0x0100 // copy RD
	}
	if !inZone {
		flags |= rCodeRefused
	} else if !nameExists {
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

	// TTL
	ttl := make([]byte, 4)
	binary.BigEndian.PutUint32(ttl, record.TTL)
	response = append(response, ttl...)

	// RDLENGTH
	rDataLength := make([]byte, 2)
	binary.BigEndian.PutUint16(rDataLength, uint16(len(record.RData)))
	response = append(response, rDataLength...)

	// RDATA = configured IPv4 address
	response = append(response, record.RData...)

	return response, nil
}

func canonicalName(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
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
