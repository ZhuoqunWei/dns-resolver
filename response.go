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
	plan := planResponse(question, zone)

	encodedName, err := encodeQName(question.Name)
	if err != nil {
		return nil, fmt.Errorf("encode qname: %w", err)
	}
	if len(plan.Answers) > 0xffff {
		return nil, fmt.Errorf("too many answer records: %d", len(plan.Answers))
	}
	if len(plan.Authorities) > 0xffff {
		return nil, fmt.Errorf("too many authority records: %d", len(plan.Authorities))
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
	var flags uint16 = 0x8000 // QR = 1

	if msg.Flags.RD {
		flags |= 0x0100 // copy RD
	}
	if plan.Authoritative {
		flags |= 0x0400 // AA = 1
	}
	flags |= plan.RCode

	flagsBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(flagsBytes, flags)
	response = append(response, flagsBytes...)

	// QDCOUNT = 1
	response = append(response, 0x00, 0x01)

	counts := make([]byte, 4)
	binary.BigEndian.PutUint16(counts[0:2], uint16(len(plan.Answers)))
	binary.BigEndian.PutUint16(counts[2:4], uint16(len(plan.Authorities)))
	response = append(response, counts...)

	// ARCOUNT = 0
	response = append(response, 0x00, 0x00)

	// Question section
	response = append(response, encodedName...)
	questionFields := make([]byte, 4)
	binary.BigEndian.PutUint16(questionFields[0:2], question.QType)
	binary.BigEndian.PutUint16(questionFields[2:4], question.QClass)
	response = append(response, questionFields...)

	for _, answer := range plan.Answers {
		response, err = appendResourceRecord(response, answer)
		if err != nil {
			return nil, fmt.Errorf("encode answer record: %w", err)
		}
	}

	for _, authority := range plan.Authorities {
		response, err = appendResourceRecord(response, authority)
		if err != nil {
			return nil, fmt.Errorf("encode authority record: %w", err)
		}
	}

	return response, nil
}

func appendResourceRecord(response []byte, record responseRecord) ([]byte, error) {
	if len(record.Record.RData) > 0xffff {
		return nil, fmt.Errorf("RDATA exceeds 65535 bytes")
	}

	if record.UseQuestionPointer {
		response = append(response, 0xc0, 0x0c)
	} else {
		encodedName, err := encodeQName(record.Name)
		if err != nil {
			return nil, fmt.Errorf("encode owner name: %w", err)
		}
		response = append(response, encodedName...)
	}

	fields := make([]byte, 10)
	binary.BigEndian.PutUint16(fields[0:2], record.Type)
	binary.BigEndian.PutUint16(fields[2:4], ClassIN)
	binary.BigEndian.PutUint32(fields[4:8], record.Record.TTL)
	binary.BigEndian.PutUint16(fields[8:10], uint16(len(record.Record.RData)))
	response = append(response, fields...)
	response = append(response, record.Record.RData...)

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
