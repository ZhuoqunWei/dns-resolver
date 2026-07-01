package main

import (
	"testing"
)

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
			data:    []byte{0x12, 0x34, 0x01},
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

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name string
		data uint16
		want Flags
	}{
		{
			name: "recursion desired query",
			data: 0x0100,
			want: Flags{RD: true},
		},
		{
			name: "response with recursion desired and available",
			data: 0x8180,
			want: Flags{QR: true, RD: true, RA: true},
		},
		{
			name: "opcode extracted correctly",
			data: 0x0800,
			want: Flags{Opcode: 1},
		},
		{
			name: "rcode extracted correctly",
			data: 0x0005,
			want: Flags{RCode: 5},
		},
		{
			name: "response with NXDOMAIN",
			data: 0x8003,
			want: Flags{QR: true, RCode: 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlags(tt.data)

			if got != tt.want {
				t.Errorf("parseFlags() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseQName(t *testing.T) {
	/**
	tests for valid name
	tests for next offset
	tests for truncated label
	tests for missing 00
	**/
	tests := []struct {
		name    string
		data    []byte
		offset  int
		want    string
		wantOff int
		wantErr bool
	}{
		{
			name:    "valid name",
			data:    []byte{0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 0x03, 'c', 'o', 'm', 0x00},
			offset:  0,
			want:    "www.example.com",
			wantOff: 17,
			wantErr: false,
		},
		{
			name:    "next offset after valid name",
			data:    []byte{0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 0x03, 'c', 'o', 'm', 0x00, 0x01, 0x02},
			offset:  0,
			want:    "www.example.com",
			wantOff: 17,
			wantErr: false,
		},
		{
			name:    "truncated label",
			data:    []byte{0x03, 'w', 'w'},
			offset:  0,
			want:    "",
			wantOff: -1,
			wantErr: true,
		},
		{
			name:    "missing 00 terminator",
			data:    []byte{0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 0x03, 'c', 'o', 'm'},
			offset:  0,
			want:    "",
			wantOff: -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOff, err := parseQName(tt.data, tt.offset)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseQName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("parseQName() = %v, want %v", got, tt.want)
			}

			if gotOff != tt.wantOff {
				t.Errorf("parseQName() offset = %v, want %v", gotOff, tt.wantOff)
			}
		})
	}
}
func TestParseQuestion(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		offset  int
		want    Question
		wantOff int
		wantErr bool
	}{
		{
			name: "valid question www.example.com A IN",
			data: []byte{
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
				0x00, 0x01, // QTYPE = 1
				0x00, 0x01, // QCLASS = 1
			},
			offset: 0,
			want: Question{
				Name:   "www.example.com",
				QType:  1,
				QClass: 1,
			},
			wantOff: 21,
			wantErr: false,
		},
		{
			name: "valid question with nonzero starting offset",
			data: []byte{
				0xaa, 0xbb, // pretend earlier bytes

				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
				0x00, 0x01, // QTYPE = 1
				0x00, 0x01, // QCLASS = 1
			},
			offset: 2,
			want: Question{
				Name:   "www.example.com",
				QType:  1,
				QClass: 1,
			},
			wantOff: 23,
			wantErr: false,
		},
		{
			name: "short QTYPE returns error",
			data: []byte{
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
				0x00, // only 1 byte of QTYPE, need 2
			},
			offset:  0,
			want:    Question{},
			wantOff: -1,
			wantErr: true,
		},
		{
			name: "short QCLASS returns error",
			data: []byte{
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,
				0x00, 0x01, // QTYPE complete
				0x00, // only 1 byte of QCLASS, need 2
			},
			offset:  0,
			want:    Question{},
			wantOff: -1,
			wantErr: true,
		},
		{
			name: "bad qname returns error",
			data: []byte{
				0x03, 'w', 'w', // says length 3, only has 2 label bytes
				0x00, 0x01,
				0x00, 0x01,
			},
			offset:  0,
			want:    Question{},
			wantOff: -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOff, err := parseQuestion(tt.data, tt.offset)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseQuestion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("parseQuestion() = %+v, want %+v", got, tt.want)
			}

			if gotOff != tt.wantOff {
				t.Errorf("parseQuestion() offset = %v, want %v", gotOff, tt.wantOff)
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    Message
		wantErr bool
	}{
		{
			name: "valid DNS query message for www.example.com A IN",
			data: []byte{
				// Header: 12 bytes
				0x12, 0x34, // ID = 0x1234 = 4660
				0x01, 0x00, // Flags = RD
				0x00, 0x01, // QDCount = 1
				0x00, 0x00, // ANCount = 0
				0x00, 0x00, // NSCount = 0
				0x00, 0x00, // ARCount = 0

				// Question
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,

				0x00, 0x01, // QTYPE = 1 = A
				0x00, 0x01, // QCLASS = 1 = IN
			},
			want: Message{
				Header: Header{
					ID:      0x1234,
					Flags:   0x0100,
					QDCount: 1,
					ANCount: 0,
					NSCount: 0,
					ARCount: 0,
				},
				Flags: Flags{
					RD: true,
				},
				Question: Question{
					Name:   "www.example.com",
					QType:  1,
					QClass: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "short header returns error",
			data: []byte{
				0x12, 0x34,
			},
			want:    Message{},
			wantErr: true,
		},
		{
			name: "bad question missing qname terminator returns error",
			data: []byte{
				// Header
				0x12, 0x34,
				0x01, 0x00,
				0x00, 0x01,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,

				// Bad QNAME: missing final 0x00
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',

				// QTYPE/QCLASS bytes exist, but parseQName should fail before them
				0x00, 0x01,
				0x00, 0x01,
			},
			want:    Message{},
			wantErr: true,
		},
		{
			name: "short QTYPE returns error",
			data: []byte{
				// Header
				0x12, 0x34,
				0x01, 0x00,
				0x00, 0x01,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,

				// QNAME
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,

				// Short QTYPE: only one byte
				0x00,
			},
			want:    Message{},
			wantErr: true,
		},
		{
			name: "short QCLASS returns error",
			data: []byte{
				// Header
				0x12, 0x34,
				0x01, 0x00,
				0x00, 0x01,
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,

				// QNAME
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,

				0x00, 0x01, // QTYPE complete

				// Short QCLASS: only one byte
				0x00,
			},
			want:    Message{},
			wantErr: true,
		},
		{
			name: "non-one QDCount returns error",
			data: []byte{
				// Header
				0x12, 0x34,
				0x01, 0x00,
				0x00, 0x02, // QDCount = 2
				0x00, 0x00,
				0x00, 0x00,
				0x00, 0x00,

				// One Question only
				0x03, 'w', 'w', 'w',
				0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				0x03, 'c', 'o', 'm',
				0x00,

				0x00, 0x01,
				0x00, 0x01,
			},
			want:    Message{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMessage(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("parseMessage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
