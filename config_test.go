package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSOAConfig = `"soa": {
	"nameServer": "ns1.example.com",
	"responsibleName": "hostmaster.example.com",
	"serial": 2026072501,
	"refresh": 3600,
	"retry": 600,
	"expire": 86400,
	"minimum": 120,
	"ttl": 300
}`

func writeRecordsConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write records config: %v", err)
	}

	return path
}

func validRecordsConfig(records string) string {
	return fmt.Sprintf(`{"origin": "example.com", %s, "records": %s}`, validSOAConfig, records)
}

func TestLoadZone(t *testing.T) {
	path := writeRecordsConfig(t, fmt.Sprintf(`{
		"origin": "Example.COM.",
		%s,
		"records": [
			{"name": "Example.COM.", "type": "A", "value": "1.2.3.4", "ttl": 60},
			{"name": "Example.COM.", "type": "aaaa", "value": "2001:db8::1", "ttl": 120},
			{"name": "test.example.com", "type": "A", "value": "5.6.7.8", "ttl": 300},
			{"name": "pool.example.com", "type": "A", "value": "192.0.2.10", "ttl": 90},
			{"name": "POOL.EXAMPLE.COM.", "type": "a", "value": "192.0.2.11", "ttl": 90}
		]
	}`, validSOAConfig))

	zone, err := loadZone(path)
	if err != nil {
		t.Fatalf("loadZone returned error: %v", err)
	}
	if zone.Origin != "example.com" {
		t.Fatalf("zone origin = %q, want %q", zone.Origin, "example.com")
	}

	exampleRecords, exists := zone.Records["example.com"]
	if !exists {
		t.Fatal(`records["example.com"] does not exist`)
	}
	if len(exampleRecords[TypeA]) != 1 {
		t.Fatalf("example.com A record count = %d, want 1", len(exampleRecords[TypeA]))
	}
	example := exampleRecords[TypeA][0]
	if example.TTL != 60 {
		t.Fatalf("example.com TTL = %d, want 60", example.TTL)
	}
	if !bytes.Equal(example.RData, []byte{1, 2, 3, 4}) {
		t.Fatalf("example.com RDATA = %v, want [1 2 3 4]", example.RData)
	}
	if len(exampleRecords[TypeAAAA]) != 1 {
		t.Fatalf("example.com AAAA record count = %d, want 1", len(exampleRecords[TypeAAAA]))
	}
	exampleAAAA := exampleRecords[TypeAAAA][0]
	if exampleAAAA.TTL != 120 {
		t.Fatalf("example.com AAAA TTL = %d, want 120", exampleAAAA.TTL)
	}
	wantAAAA := []byte{
		0x20, 0x01, 0x0d, 0xb8,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
	}
	if !bytes.Equal(exampleAAAA.RData, wantAAAA) {
		t.Fatalf("example.com AAAA RDATA = %v, want %v", exampleAAAA.RData, wantAAAA)
	}
	if len(exampleRecords[TypeSOA]) != 1 {
		t.Fatalf("example.com SOA record count = %d, want 1", len(exampleRecords[TypeSOA]))
	}
	soa := exampleRecords[TypeSOA][0]
	if soa.TTL != 300 {
		t.Fatalf("example.com SOA TTL = %d, want 300", soa.TTL)
	}

	nameServer, offset, err := parseQName(soa.RData, 0)
	if err != nil {
		t.Fatalf("parse SOA name server: %v", err)
	}
	responsibleName, offset, err := parseQName(soa.RData, offset)
	if err != nil {
		t.Fatalf("parse SOA responsible name: %v", err)
	}
	if nameServer != "ns1.example.com" {
		t.Fatalf("SOA name server = %q, want %q", nameServer, "ns1.example.com")
	}
	if responsibleName != "hostmaster.example.com" {
		t.Fatalf("SOA responsible name = %q, want %q", responsibleName, "hostmaster.example.com")
	}
	if len(soa.RData) != offset+20 {
		t.Fatalf("SOA RDATA length = %d, want %d", len(soa.RData), offset+20)
	}
	wantSOAValues := [...]uint32{2026072501, 3600, 600, 86400, 120}
	for i, want := range wantSOAValues {
		got := binary.BigEndian.Uint32(soa.RData[offset+i*4 : offset+(i+1)*4])
		if got != want {
			t.Fatalf("SOA value %d = %d, want %d", i, got, want)
		}
	}

	testRecords, exists := zone.Records["test.example.com"]
	if !exists {
		t.Fatal(`records["test.example.com"] does not exist`)
	}
	if len(testRecords[TypeA]) != 1 {
		t.Fatalf("test.example.com A record count = %d, want 1", len(testRecords[TypeA]))
	}
	testRecord := testRecords[TypeA][0]
	if testRecord.TTL != 300 {
		t.Fatalf("test.example.com TTL = %d, want 300", testRecord.TTL)
	}
	if !bytes.Equal(testRecord.RData, []byte{5, 6, 7, 8}) {
		t.Fatalf("test.example.com RDATA = %v, want [5 6 7 8]", testRecord.RData)
	}

	poolRecords, exists := zone.Records["pool.example.com"]
	if !exists {
		t.Fatal(`records["pool.example.com"] does not exist`)
	}
	if len(poolRecords[TypeA]) != 2 {
		t.Fatalf("pool.example.com A record count = %d, want 2", len(poolRecords[TypeA]))
	}
	wantPoolRData := [][]byte{
		{192, 0, 2, 10},
		{192, 0, 2, 11},
	}
	for i, record := range poolRecords[TypeA] {
		if record.TTL != 90 {
			t.Fatalf("pool.example.com record %d TTL = %d, want 90", i, record.TTL)
		}
		if !bytes.Equal(record.RData, wantPoolRData[i]) {
			t.Fatalf("pool.example.com record %d RDATA = %v, want %v", i, record.RData, wantPoolRData[i])
		}
	}
}

func TestLoadZoneRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError string
	}{
		{
			name:    "malformed JSON",
			content: `{"records": [`,
		},
		{
			name:    "unknown field",
			content: fmt.Sprintf(`{"origin": "example.com", %s, "records": [], "unexpected": true}`, validSOAConfig),
		},
		{
			name:    "missing origin",
			content: fmt.Sprintf(`{%s, "records": []}`, validSOAConfig),
		},
		{
			name:    "invalid origin",
			content: fmt.Sprintf(`{"origin": "example..com", %s, "records": []}`, validSOAConfig),
		},
		{
			name:    "missing SOA",
			content: `{"origin": "example.com", "records": []}`,
		},
		{
			name: "invalid SOA name server",
			content: `{
				"origin": "example.com",
				"soa": {
					"nameServer": "ns1..example.com",
					"responsibleName": "hostmaster.example.com",
					"serial": 1,
					"refresh": 3600,
					"retry": 600,
					"expire": 86400,
					"minimum": 120,
					"ttl": 300
				},
				"records": []
			}`,
		},
		{
			name: "missing SOA responsible name",
			content: `{
				"origin": "example.com",
				"soa": {
					"nameServer": "ns1.example.com",
					"serial": 1,
					"refresh": 3600,
					"retry": 600,
					"expire": 86400,
					"minimum": 120,
					"ttl": 300
				},
				"records": []
			}`,
		},
		{
			name:    "empty name",
			content: validRecordsConfig(`[{"name": "", "type": "A", "value": "1.2.3.4", "ttl": 60}]`),
		},
		{
			name:    "invalid name",
			content: validRecordsConfig(`[{"name": "example..com", "type": "A", "value": "1.2.3.4", "ttl": 60}]`),
		},
		{
			name:    "record outside zone",
			content: validRecordsConfig(`[{"name": "other.com", "type": "A", "value": "1.2.3.4", "ttl": 60}]`),
		},
		{
			name:    "unsupported type",
			content: validRecordsConfig(`[{"name": "example.com", "type": "TXT", "value": "hello", "ttl": 60}]`),
		},
		{
			name:    "invalid A value",
			content: validRecordsConfig(`[{"name": "example.com", "type": "A", "value": "not-an-ip", "ttl": 60}]`),
		},
		{
			name:    "IPv6 value for A",
			content: validRecordsConfig(`[{"name": "example.com", "type": "A", "value": "2001:db8::1", "ttl": 60}]`),
		},
		{
			name:    "IPv4 value for AAAA",
			content: validRecordsConfig(`[{"name": "example.com", "type": "AAAA", "value": "1.2.3.4", "ttl": 60}]`),
		},
		{
			name:      "duplicate canonical name type and value",
			wantError: "duplicate A value",
			content: validRecordsConfig(`[
					{"name": "example.com", "type": "A", "value": "1.2.3.4", "ttl": 60},
					{"name": "EXAMPLE.COM.", "type": "a", "value": "1.2.3.4", "ttl": 60}
				]`),
		},
		{
			name:      "inconsistent TTL in RRset",
			wantError: "must use the same TTL",
			content: validRecordsConfig(`[
					{"name": "example.com", "type": "A", "value": "1.2.3.4", "ttl": 60},
					{"name": "EXAMPLE.COM.", "type": "a", "value": "5.6.7.8", "ttl": 300}
				]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeRecordsConfig(t, tt.content)

			_, err := loadZone(path)
			if err == nil {
				t.Fatal("loadZone returned nil error")
			}
			if tt.wantError != "" && !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("loadZone error = %q, want it to contain %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadZoneReturnsReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	if _, err := loadZone(path); err == nil {
		t.Fatal("loadZone returned nil error for missing file")
	}
}
