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
			{"name": "Example.COM.", "address": "1.2.3.4", "ttl": 60},
			{"name": "test.example.com", "address": "5.6.7.8", "ttl": 300}
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
			content: `{"origin": "example.com", "records": [{"name": "example.com", "address": "1.2.3.4", "tll": 60}]}`,
		},
		{
			name:    "missing origin",
			content: `{"records": [{"name": "example.com", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid origin",
			content: `{"origin": "example..com", "records": []}`,
		},
		{
			name:    "empty name",
			content: `{"origin": "example.com", "records": [{"name": "", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid name",
			content: `{"origin": "example.com", "records": [{"name": "example..com", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "record outside zone",
			content: `{"origin": "example.com", "records": [{"name": "other.com", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid address",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "address": "not-an-ip", "ttl": 60}]}`,
		},
		{
			name:    "IPv6 address",
			content: `{"origin": "example.com", "records": [{"name": "example.com", "address": "2001:db8::1", "ttl": 60}]}`,
		},
		{
			name: "duplicate canonical name",
			content: `{
				"origin": "example.com",
				"records": [
					{"name": "example.com", "address": "1.2.3.4", "ttl": 60},
					{"name": "EXAMPLE.COM.", "address": "5.6.7.8", "ttl": 300}
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
