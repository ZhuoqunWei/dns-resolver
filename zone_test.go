package main

import (
	"encoding/binary"
	"testing"
)

func testSOARData() []byte {
	nameServer, err := encodeQName("ns1.example.com")
	if err != nil {
		panic(err)
	}
	responsibleName, err := encodeQName("hostmaster.example.com")
	if err != nil {
		panic(err)
	}

	rData := append(nameServer, responsibleName...)
	values := [...]uint32{2026072501, 3600, 600, 86400, 120}
	for _, value := range values {
		var encoded [4]byte
		binary.BigEndian.PutUint32(encoded[:], value)
		rData = append(rData, encoded[:]...)
	}

	return rData
}

func testZone() Zone {
	return Zone{
		Origin: "example.com",
		Records: map[string]map[uint16][]Record{
			"example.com": {
				TypeA: {
					{
						TTL:   60,
						RData: []byte{1, 2, 3, 4},
					},
				},
				TypeAAAA: {
					{
						TTL: 120,
						RData: []byte{
							0x20, 0x01, 0x0d, 0xb8,
							0x00, 0x00, 0x00, 0x00,
							0x00, 0x00, 0x00, 0x00,
							0x00, 0x00, 0x00, 0x01,
						},
					},
				},
				TypeSOA: {
					{
						TTL:   300,
						RData: testSOARData(),
					},
				},
			},
			"test.example.com": {
				TypeA: {
					{
						TTL:   300,
						RData: []byte{5, 6, 7, 8},
					},
				},
			},
			"pool.example.com": {
				TypeA: {
					{
						TTL:   90,
						RData: []byte{192, 0, 2, 10},
					},
					{
						TTL:   90,
						RData: []byte{192, 0, 2, 11},
					},
				},
			},
		},
	}
}

func testZoneWithLargeARRset(recordCount int) Zone {
	zone := testZone()
	records := make([]Record, recordCount)
	for i := range records {
		records[i] = Record{
			TTL:   90,
			RData: []byte{192, 0, 2, byte(i + 1)},
		}
	}
	zone.Records["pool.example.com"][TypeA] = records

	return zone
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
		{name: "pool.example.com", want: true},
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
