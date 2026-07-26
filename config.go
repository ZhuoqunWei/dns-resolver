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
	Origin  string          `json:"origin"`
	Records []aRecordConfig `json:"records"`
}

type aRecordConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	TTL     uint32 `json:"ttl"`
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
		if len(recordsByType[TypeA]) > 0 {
			return Zone{}, fmt.Errorf("record %d: duplicate name %q", recordNumber, name)
		}

		address, err := netip.ParseAddr(strings.TrimSpace(configuredRecord.Address))
		if err != nil {
			return Zone{}, fmt.Errorf("record %d: invalid address %q: %w", recordNumber, configuredRecord.Address, err)
		}
		if !address.Is4() {
			return Zone{}, fmt.Errorf("record %d: address %q is not IPv4", recordNumber, configuredRecord.Address)
		}

		ipv4 := address.As4()
		recordsByType[TypeA] = []Record{
			{
				TTL:   configuredRecord.TTL,
				RData: append([]byte(nil), ipv4[:]...),
			},
		}
	}

	return zone, nil
}
