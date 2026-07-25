package main

import (
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

func TestLoadARecords(t *testing.T) {
	path := writeRecordsConfig(t, `{
		"records": [
			{"name": "Example.COM.", "address": "1.2.3.4", "ttl": 60},
			{"name": "test.example.com", "address": "5.6.7.8", "ttl": 300}
		]
	}`)

	records, err := loadARecords(path)
	if err != nil {
		t.Fatalf("loadARecords returned error: %v", err)
	}

	example, exists := records["example.com"]
	if !exists {
		t.Fatal(`records["example.com"] does not exist`)
	}
	if example.Address != [4]byte{1, 2, 3, 4} {
		t.Fatalf("example.com address = %v, want [1 2 3 4]", example.Address)
	}
	if example.TTL != 60 {
		t.Fatalf("example.com TTL = %d, want 60", example.TTL)
	}

	testRecord, exists := records["test.example.com"]
	if !exists {
		t.Fatal(`records["test.example.com"] does not exist`)
	}
	if testRecord.Address != [4]byte{5, 6, 7, 8} {
		t.Fatalf("test.example.com address = %v, want [5 6 7 8]", testRecord.Address)
	}
	if testRecord.TTL != 300 {
		t.Fatalf("test.example.com TTL = %d, want 300", testRecord.TTL)
	}
}

func TestLoadARecordsRejectsInvalidConfiguration(t *testing.T) {
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
			content: `{"records": [{"name": "example.com", "address": "1.2.3.4", "tll": 60}]}`,
		},
		{
			name:    "empty name",
			content: `{"records": [{"name": "", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid name",
			content: `{"records": [{"name": "example..com", "address": "1.2.3.4", "ttl": 60}]}`,
		},
		{
			name:    "invalid address",
			content: `{"records": [{"name": "example.com", "address": "not-an-ip", "ttl": 60}]}`,
		},
		{
			name:    "IPv6 address",
			content: `{"records": [{"name": "example.com", "address": "2001:db8::1", "ttl": 60}]}`,
		},
		{
			name: "duplicate canonical name",
			content: `{
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

			if _, err := loadARecords(path); err == nil {
				t.Fatal("loadARecords returned nil error")
			}
		})
	}
}

func TestLoadARecordsReturnsReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	if _, err := loadARecords(path); err == nil {
		t.Fatal("loadARecords returned nil error for missing file")
	}
}
