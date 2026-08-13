package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	maxDNSUDPMessageSize  = 512
	maxEDNSUDPMessageSize = 1232
	maxDNSTCPMessageSize  = 0xffff

	rCodeNXDomain uint16 = 3
	rCodeRefused  uint16 = 5

	extendedRCodeBadVersion uint8 = 1
)

func buildResponse(msg Message, zone Zone) ([]byte, error) {
	return buildResponseWithLimit(msg, zone, maxDNSUDPMessageSize)
}

func buildResponseWithLimit(msg Message, zone Zone, sizeLimit int) ([]byte, error) {
	question := msg.Question
	plan := planResponse(question, zone)
	var extendedRCode uint8
	if msg.EDNS != nil && msg.EDNS.Version != 0 {
		plan = responsePlan{}
		extendedRCode = extendedRCodeBadVersion
	}

	encodedName, err := encodeQName(question.Name)
	if err != nil {
		return nil, fmt.Errorf("encode qname: %w", err)
	}
	if sizeLimit <= 0 {
		return nil, fmt.Errorf("response size limit must be positive")
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

	// ANCOUNT and NSCOUNT are patched after complete records are appended.
	response = append(response, 0x00, 0x00, 0x00, 0x00)

	// ARCOUNT = 0
	response = append(response, 0x00, 0x00)

	// Question section
	response = append(response, encodedName...)
	questionFields := make([]byte, 4)
	binary.BigEndian.PutUint16(questionFields[0:2], question.QType)
	binary.BigEndian.PutUint16(questionFields[2:4], question.QClass)
	response = append(response, questionFields...)

	var optRecord []byte
	recordsLimit := sizeLimit
	if msg.EDNS != nil {
		optRecord = encodeOPTRecord(maxEDNSUDPMessageSize, extendedRCode)
		recordsLimit -= len(optRecord)
	}

	if len(response) > recordsLimit {
		return nil, fmt.Errorf(
			"response header, question, and required metadata need %d bytes, exceeding size limit %d",
			len(response)+len(optRecord),
			sizeLimit,
		)
	}

	var answerCount uint16
	var authorityCount uint16
	var truncated bool

	response, answerCount, truncated, err = appendRecordsWithinLimit(
		response,
		plan.Answers,
		recordsLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("encode answer records: %w", err)
	}

	if !truncated {
		var authorityTruncated bool
		response, authorityCount, authorityTruncated, err = appendRecordsWithinLimit(
			response,
			plan.Authorities,
			recordsLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("encode authority records: %w", err)
		}
		truncated = authorityTruncated
	}

	var additionalCount uint16
	if msg.EDNS != nil {
		response = append(response, optRecord...)
		additionalCount = 1
	}

	if truncated {
		flags |= 0x0200 // TC = 1
	}
	binary.BigEndian.PutUint16(response[2:4], flags)
	binary.BigEndian.PutUint16(response[6:8], answerCount)
	binary.BigEndian.PutUint16(response[8:10], authorityCount)
	binary.BigEndian.PutUint16(response[10:12], additionalCount)

	return response, nil
}

func encodeOPTRecord(udpPayloadSize uint16, extendedRCode uint8) []byte {
	record := make([]byte, 11)
	record[0] = 0x00 // root owner name
	binary.BigEndian.PutUint16(record[1:3], TypeOPT)
	binary.BigEndian.PutUint16(record[3:5], udpPayloadSize)
	record[5] = extendedRCode
	// EDNS version, flags, and RDLENGTH remain zero.
	return record
}

func appendRecordsWithinLimit(
	response []byte,
	records []responseRecord,
	sizeLimit int,
) ([]byte, uint16, bool, error) {
	var count uint16

	for _, record := range records {
		if count == 0xffff {
			return response, count, true, nil
		}

		encoded, err := appendResourceRecord(nil, record)
		if err != nil {
			return nil, 0, false, err
		}
		if len(response)+len(encoded) > sizeLimit {
			return response, count, true, nil
		}

		response = append(response, encoded...)
		count++
	}

	return response, count, false, nil
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
