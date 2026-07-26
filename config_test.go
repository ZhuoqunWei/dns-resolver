package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeRecordsConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write records config: %v", err)
	}

	return path
}

func TestLoadZone(t *testing.T) {
	path := writeRecordsConfig(t, `{
		"origin": "Example.COM.",
		"records": [
			{"name": "Example.COM.", "type": "A", "value": "1.2.3.4", "ttl": 60},
			{"name": "Example.COM.", "type": "aaaa", "value": "2001:db8::1", "ttl": 120},
			{"name": "test.example.com", "type": "A", "value": "5.6.7.8", "ttl": 300}
		]
	}`)

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
}

func TestLoadZoneRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "malformed JSON",
			content: `{"records": [`,
		},
		{
			name:    "unknown field",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "type": "A", "value": "1.2.3.4", "tll": 60}]}`,
		},
		{
			name:    "missing origin",
			content: `{"records": [{"name": "example.com", "type": "A", "value": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid origin",
			content: `{"origin": "example..com", "records": []}`,
		},
		{
			name:    "empty name",
			content: `{"origin": "example.com", "records": [{"name": "", "type": "A", "value": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid name",
			content: `{"origin": "example.com", "records": [{"name": "example..com", "type": "A", "value": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "record outside zone",
			content: `{"origin": "example.com", "records": [{"name": "other.com", "type": "A", "value": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "unsupported type",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "type": "TXT", "value": "hello", "ttl": 60}]}`,
		},
		{
			name:    "invalid A value",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "type": "A", "value": "not-an-ip", "ttl": 60}]}`,
		},
		{
			name:    "IPv6 value for A",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "type": "A", "value": "2001:db8::1", "ttl": 60}]}`,
		},
		{
			name:    "IPv4 value for AAAA",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "type": "AAAA", "value": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name: "duplicate canonical name and type",
			content: `{
				"origin": "example.com",
				"records": [
					{"name": "example.com", "type": "A", "value": "1.2.3.4", "ttl": 60},
					{"name": "EXAMPLE.COM.", "type": "a", "value": "5.6.7.8", "ttl": 300}
				]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeRecordsConfig(t, tt.content)

			if _, err := loadZone(path); err == nil {
				t.Fatal("loadZone returned nil error")
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
