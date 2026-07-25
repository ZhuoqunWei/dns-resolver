package main

import "testing"

func testZone() Zone {
	return Zone{
		Origin: "example.com",
		Records: map[string]ARecord{
			"example.com": {
				Address: [4]byte{1, 2, 3, 4},
				TTL:     60,
			},
			"test.example.com": {
				Address: [4]byte{5, 6, 7, 8},
				TTL:     300,
			},
		},
	}
}

func TestZoneContains(t *testing.T) {
	zone := testZone()

	tests := []struct {
		name string
		want bool
	}{
		{name: "example.com", want: true},
		{name: "www.example.com", want: true},
		{name: "WWW.EXAMPLE.COM.", want: true},
		{name: "other.com", want: false},
		{name: "badexample.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zone.contains(tt.name); got != tt.want {
				t.Fatalf("zone.contains(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestZoneNameExists(t *testing.T) {
	zone := testZone()

	tests := []struct {
		name string
		want bool
	}{
		{name: "example.com", want: true},
		{name: "test.example.com", want: true},
		{name: "missing.example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zone.nameExists(tt.name); got != tt.want {
				t.Fatalf("zone.nameExists(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
