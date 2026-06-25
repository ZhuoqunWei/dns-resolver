package main

import "testing"

func TestReadU16(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		offset  int
		want    uint16
		wantErr bool
	}{
		{"reads 0x1234", []byte{0x12, 0x34}, 0, 4660, false},
		{"reads 0x0001", []byte{0x00, 0x01}, 0, 1, false},
		{"reads from offset 1", []byte{0x12, 0x34, 0x56}, 1, 13398, false},
		{"reads from offset 1 with extra byte", []byte{0x12, 0x34, 0x56, 0x78}, 1, 13398, false},
		{"errors on short input", []byte{0x12}, 0, 0, true},
		{"negative offset", []byte{0x12, 0x34}, -1, 0, true},
		{"offset out of bounds", []byte{0x12, 0x34}, 2, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readU16(tt.data, tt.offset)

			if (err != nil) != tt.wantErr {
				t.Errorf("readU16() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("readU16() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    Header
		wantErr bool
	}{
		{
			name:    "valid header",
			data:    []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04},
			want:    Header{ID: 4660, Flags: 256, QDCount: 1, ANCount: 2, NSCount: 3, ARCount: 4},
			wantErr: false,
		},
		{
			name:    "short data",
			data:    []byte{0x12, 0x34},
			want:    Header{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHeader(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("parseHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
