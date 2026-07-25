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

type recordsConfig struct {
	Records []aRecordConfig `json:"records"`
}

type aRecordConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	TTL     uint32 `json:"ttl"`
}

func loadARecords(path string) (map[string]ARecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read records config %q: %w", path, err)
	}

	var config recordsConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("parse records config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("parse records config %q: expected one JSON object", path)
	}

	records := make(map[string]ARecord, len(config.Records))
	for i, configuredRecord := range config.Records {
		recordNumber := i + 1
		name := canonicalName(strings.TrimSpace(configuredRecord.Name))
		if name == "" {
			return nil, fmt.Errorf("record %d: name is required", recordNumber)
		}
		if _, err := encodeQName(name); err != nil {
			return nil, fmt.Errorf("record %d: invalid name %q: %w", recordNumber, configuredRecord.Name, err)
		}
		if _, exists := records[name]; exists {
			return nil, fmt.Errorf("record %d: duplicate name %q", recordNumber, name)
		}

		address, err := netip.ParseAddr(strings.TrimSpace(configuredRecord.Address))
		if err != nil {
			return nil, fmt.Errorf("record %d: invalid address %q: %w", recordNumber, configuredRecord.Address, err)
		}
		if !address.Is4() {
			return nil, fmt.Errorf("record %d: address %q is not IPv4", recordNumber, configuredRecord.Address)
		}

		records[name] = ARecord{
			Address: address.As4(),
			TTL:     configuredRecord.TTL,
		}
	}

	return records, nil
}
