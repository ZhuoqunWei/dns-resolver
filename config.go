package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
)

type zoneConfig struct {
	Origin  string         `json:"origin"`
	Records []recordConfig `json:"records"`
}

type recordConfig struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl"`
}

func loadZone(path string) (Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Zone{}, fmt.Errorf("read zone config %q: %w", path, err)
	}

	var config zoneConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Zone{}, fmt.Errorf("parse zone config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Zone{}, fmt.Errorf("parse zone config %q: expected one JSON object", path)
	}

	origin := canonicalName(strings.TrimSpace(config.Origin))
	if origin == "" {
		return Zone{}, fmt.Errorf("zone origin is required")
	}
	if _, err := encodeQName(origin); err != nil {
		return Zone{}, fmt.Errorf("invalid zone origin %q: %w", config.Origin, err)
	}

	zone := Zone{
		Origin:  origin,
		Records: make(map[string]map[uint16][]Record, len(config.Records)),
	}

	for i, configuredRecord := range config.Records {
		recordNumber := i + 1
		name := canonicalName(strings.TrimSpace(configuredRecord.Name))
		if name == "" {
			return Zone{}, fmt.Errorf("record %d: name is required", recordNumber)
		}
		if _, err := encodeQName(name); err != nil {
			return Zone{}, fmt.Errorf("record %d: invalid name %q: %w", recordNumber, configuredRecord.Name, err)
		}
		if !zone.contains(name) {
			return Zone{}, fmt.Errorf("record %d: name %q is outside zone %q", recordNumber, name, zone.Origin)
		}
		recordsByType, exists := zone.Records[name]
		if !exists {
			recordsByType = make(map[uint16][]Record)
			zone.Records[name] = recordsByType
		}

		recordType, rData, err := parseAddressRecord(configuredRecord.Type, configuredRecord.Value)
		if err != nil {
			return Zone{}, fmt.Errorf("record %d: %w", recordNumber, err)
		}
		if len(recordsByType[recordType]) > 0 {
			return Zone{}, fmt.Errorf(
				"record %d: duplicate %s record for name %q",
				recordNumber,
				strings.ToUpper(strings.TrimSpace(configuredRecord.Type)),
				name,
			)
		}

		recordsByType[recordType] = []Record{
			{
				TTL:   configuredRecord.TTL,
				RData: rData,
			},
		}
	}

	return zone, nil
}

func parseAddressRecord(recordType string, value string) (uint16, []byte, error) {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	value = strings.TrimSpace(value)

	switch recordType {
	case "A":
		address, err := netip.ParseAddr(value)
		if err != nil {
			return 0, nil, fmt.Errorf("invalid A value %q: %w", value, err)
		}
		if !address.Is4() {
			return 0, nil, fmt.Errorf("A value %q is not IPv4", value)
		}

		ipv4 := address.As4()
		return TypeA, append([]byte(nil), ipv4[:]...), nil

	case "AAAA":
		address, err := netip.ParseAddr(value)
		if err != nil {
			return 0, nil, fmt.Errorf("invalid AAAA value %q: %w", value, err)
		}
		if !address.Is6() {
			return 0, nil, fmt.Errorf("AAAA value %q is not IPv6", value)
		}

		ipv6 := address.As16()
		return TypeAAAA, append([]byte(nil), ipv6[:]...), nil

	default:
		return 0, nil, fmt.Errorf("unsupported record type %q", recordType)
	}
}
