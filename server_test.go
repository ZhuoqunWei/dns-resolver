package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestHandlePacketRejectsMalformedQuery(t *testing.T) {
	if _, _, err := handlePacket([]byte{0x00}, testZone()); err == nil {
		t.Fatal("handlePacket returned nil error for malformed query")
	}
}

func TestServeUDPRespondsToQueries(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatalf("listen for UDP: %v", err)
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveUDP(serverConn, testZone(), io.Discard)
	}()

	clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		serverConn.Close()
		t.Fatalf("dial UDP server: %v", err)
	}
	defer clientConn.Close()

	tests := []struct {
		name        string
		query       []byte
		wantANCount uint16
		wantNSCount uint16
		wantRCode   uint16
		wantAA      bool
		wantType    uint16
		wantRData   [][]byte
	}{
		{
			name:        "configured A record",
			query:       sampleQueryWithTypeClass(TypeA, ClassIN),
			wantANCount: 1,
			wantAA:      true,
			wantType:    TypeA,
			wantRData:   [][]byte{{1, 2, 3, 4}},
		},
		{
			name:        "configured AAAA record",
			query:       sampleQueryWithTypeClass(TypeAAAA, ClassIN),
			wantANCount: 1,
			wantAA:      true,
			wantType:    TypeAAAA,
			wantRData: [][]byte{
				{
					0x20, 0x01, 0x0d, 0xb8,
					0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x01,
				},
			},
		},
		{
			name:        "configured SOA record",
			query:       sampleQueryWithTypeClass(TypeSOA, ClassIN),
			wantANCount: 1,
			wantAA:      true,
			wantType:    TypeSOA,
			wantRData:   [][]byte{testSOARData()},
		},
		{
			name:        "configured A RRset",
			query:       samplePoolExampleAQuery(),
			wantANCount: 2,
			wantAA:      true,
			wantType:    TypeA,
			wantRData: [][]byte{
				{192, 0, 2, 10},
				{192, 0, 2, 11},
			},
		},
		{
			name:        "unsupported type at existing name",
			query:       sampleQueryWithTypeClass(TypeTXT, ClassIN),
			wantNSCount: 1,
			wantAA:      true,
		},
		{
			name:        "missing in-zone name",
			query:       sampleMissingSubdomainAQuery(),
			wantNSCount: 1,
			wantRCode:   rCodeNXDomain,
			wantAA:      true,
		},
		{
			name:      "out-of-zone name",
			query:     sampleOtherDomainAQuery(),
			wantRCode: rCodeRefused,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := clientConn.SetDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatalf("set UDP deadline: %v", err)
			}

			if _, err := clientConn.Write(tt.query); err != nil {
				t.Fatalf("write UDP query: %v", err)
			}

			buf := make([]byte, 512)
			n, err := clientConn.Read(buf)
			if err != nil {
				t.Fatalf("read UDP response: %v", err)
			}
			response := buf[:n]

			if len(response) < HeaderSize {
				t.Fatalf("response length = %d, want at least %d", len(response), HeaderSize)
			}
			if !bytes.Equal(response[0:2], tt.query[0:2]) {
				t.Fatalf("response ID = %v, want %v", response[0:2], tt.query[0:2])
			}

			anCount := binary.BigEndian.Uint16(response[6:8])
			if anCount != tt.wantANCount {
				t.Fatalf("ANCOUNT = %d, want %d", anCount, tt.wantANCount)
			}
			nsCount := binary.BigEndian.Uint16(response[8:10])
			if nsCount != tt.wantNSCount {
				t.Fatalf("NSCOUNT = %d, want %d", nsCount, tt.wantNSCount)
			}

			flags := binary.BigEndian.Uint16(response[2:4])
			rCode := flags & 0x000f
			if rCode != tt.wantRCode {
				t.Fatalf("RCODE = %d, want %d", rCode, tt.wantRCode)
			}
			authoritative := flags&0x0400 != 0
			if authoritative != tt.wantAA {
				t.Fatalf("AA = %t, want %t; flags=%016b", authoritative, tt.wantAA, flags)
			}

			if tt.wantRData != nil {
				offset := len(tt.query)
				for i, wantRData := range tt.wantRData {
					answer := parseTestResourceRecord(t, response, offset)
					if answer.Type != tt.wantType {
						t.Fatalf("answer %d TYPE = %d, want %d", i, answer.Type, tt.wantType)
					}
					if !bytes.Equal(answer.RData, wantRData) {
						t.Fatalf("answer %d RDATA = %v, want %v", i, answer.RData, wantRData)
					}
					offset = answer.Next
				}
			}
		})
	}

	if err := serverConn.Close(); err != nil {
		t.Fatalf("close UDP server: %v", err)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("serveUDP returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveUDP did not stop after its connection was closed")
	}
}

func TestServeUDPTruncatesOversizedResponse(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatalf("listen for UDP: %v", err)
	}
	defer serverConn.Close()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveUDP(serverConn, testZoneWithLargeARRset(40), io.Discard)
	}()

	clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial UDP server: %v", err)
	}
	defer clientConn.Close()

	if err := clientConn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set UDP deadline: %v", err)
	}

	query := samplePoolExampleAQuery()
	if _, err := clientConn.Write(query); err != nil {
		t.Fatalf("write UDP query: %v", err)
	}

	buf := make([]byte, 2048)
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("read UDP response: %v", err)
	}
	response := buf[:n]

	if len(response) > maxDNSUDPMessageSize {
		t.Fatalf("response length = %d, want at most %d", len(response), maxDNSUDPMessageSize)
	}
	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0200 == 0 {
		t.Fatalf("TC flag is not set for truncated UDP response; flags=%016b", flags)
	}

	const wantAnswerCount uint16 = 29
	answerCount := binary.BigEndian.Uint16(response[6:8])
	if answerCount != wantAnswerCount {
		t.Fatalf("ANCOUNT = %d, want %d", answerCount, wantAnswerCount)
	}

	offset := len(query)
	for i := 0; i < int(answerCount); i++ {
		answer := parseTestResourceRecord(t, response, offset)
		offset = answer.Next
	}
	if offset != len(response) {
		t.Fatalf("parsed response through offset %d, response length is %d", offset, len(response))
	}

	if err := serverConn.Close(); err != nil {
		t.Fatalf("close UDP server: %v", err)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("serveUDP returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveUDP did not stop after its connection was closed")
	}
}
