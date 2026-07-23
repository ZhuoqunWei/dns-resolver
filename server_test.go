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
	records := map[string][4]byte{
		"example.com": {1, 2, 3, 4},
	}

	if _, _, err := handlePacket([]byte{0x00}, records); err == nil {
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

	records := map[string][4]byte{
		"example.com": {1, 2, 3, 4},
		"test.local":  {5, 6, 7, 8},
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveUDP(serverConn, records, io.Discard)
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
		wantRCode   uint16
		wantRData   []byte
	}{
		{
			name:        "configured A record",
			query:       sampleQueryWithTypeClass(TypeA, ClassIN),
			wantANCount: 1,
			wantRData:   []byte{1, 2, 3, 4},
		},
		{
			name:      "unknown name",
			query:     sampleOtherDomainAQuery(),
			wantRCode: rCodeNXDomain,
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

			flags := binary.BigEndian.Uint16(response[2:4])
			rCode := flags & 0x000f
			if rCode != tt.wantRCode {
				t.Fatalf("RCODE = %d, want %d", rCode, tt.wantRCode)
			}

			if tt.wantRData != nil && !bytes.Equal(response[len(response)-4:], tt.wantRData) {
				t.Fatalf("RDATA = %v, want %v", response[len(response)-4:], tt.wantRData)
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
