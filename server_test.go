package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"testing/iotest"
	"time"
)

func TestHandlePacketRejectsMalformedQuery(t *testing.T) {
	if _, _, err := handlePacket([]byte{0x00}, testZone()); err == nil {
		t.Fatal("handlePacket returned nil error for malformed query")
	}
}

func TestReadTCPMessageHandlesFragmentedReads(t *testing.T) {
	frame := []byte{0x00, 0x04, 0x12, 0x34, 0x56, 0x78}
	reader := iotest.OneByteReader(bytes.NewReader(frame))

	message, err := readTCPMessage(reader)
	if err != nil {
		t.Fatalf("readTCPMessage returned error: %v", err)
	}
	want := []byte{0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(message, want) {
		t.Fatalf("message = %v, want %v", message, want)
	}
}

func TestReadTCPMessageRejectsMalformedFrames(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{name: "incomplete length", frame: []byte{0x00}},
		{name: "zero length", frame: []byte{0x00, 0x00}},
		{name: "incomplete message", frame: []byte{0x00, 0x04, 0x12, 0x34}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readTCPMessage(bytes.NewReader(tt.frame)); err == nil {
				t.Fatal("readTCPMessage returned nil error")
			}
		})
	}
}

func TestWriteTCPMessageAddsLengthPrefix(t *testing.T) {
	message := []byte{0x12, 0x34, 0x56, 0x78}
	var output bytes.Buffer

	if err := writeTCPMessage(&output, message); err != nil {
		t.Fatalf("writeTCPMessage returned error: %v", err)
	}

	want := []byte{0x00, 0x04, 0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("frame = %v, want %v", output.Bytes(), want)
	}
}

func TestWriteTCPMessageRejectsInvalidLengths(t *testing.T) {
	tests := []struct {
		name    string
		message []byte
	}{
		{name: "empty", message: nil},
		{name: "too large", message: make([]byte, maxDNSTCPMessageSize+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeTCPMessage(io.Discard, tt.message); err == nil {
				t.Fatal("writeTCPMessage returned nil error")
			}
		})
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

func TestServeTCPReturnsCompleteResponseAndReusesConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for TCP: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveTCP(listener, testZoneWithLargeARRset(40), io.Discard)
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial TCP server: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set TCP deadline: %v", err)
	}

	query := samplePoolExampleAQuery()
	response := exchangeTestTCPMessage(t, conn, query)
	if len(response) <= maxDNSUDPMessageSize {
		t.Fatalf("TCP response length = %d, want greater than %d", len(response), maxDNSUDPMessageSize)
	}

	flags := binary.BigEndian.Uint16(response[2:4])
	if flags&0x0200 != 0 {
		t.Fatalf("TC flag is set for complete TCP response; flags=%016b", flags)
	}

	const wantAnswerCount uint16 = 40
	answerCount := binary.BigEndian.Uint16(response[6:8])
	if answerCount != wantAnswerCount {
		t.Fatalf("ANCOUNT = %d, want %d", answerCount, wantAnswerCount)
	}

	offset := len(query)
	for i := 0; i < int(answerCount); i++ {
		answer := parseTestResourceRecord(t, response, offset)
		wantRData := []byte{192, 0, 2, byte(i + 1)}
		if !bytes.Equal(answer.RData, wantRData) {
			t.Fatalf("answer %d RDATA = %v, want %v", i, answer.RData, wantRData)
		}
		offset = answer.Next
	}
	if offset != len(response) {
		t.Fatalf("parsed response through offset %d, response length is %d", offset, len(response))
	}

	secondQuery := sampleQueryWithTypeClass(TypeAAAA, ClassIN)
	binary.BigEndian.PutUint16(secondQuery[0:2], 0xabcd)
	secondResponse := exchangeTestTCPMessage(t, conn, secondQuery)
	if responseID := binary.BigEndian.Uint16(secondResponse[0:2]); responseID != 0xabcd {
		t.Fatalf("second response ID = 0x%04x, want 0xabcd", responseID)
	}
	if secondAnswerCount := binary.BigEndian.Uint16(secondResponse[6:8]); secondAnswerCount != 1 {
		t.Fatalf("second response ANCOUNT = %d, want 1", secondAnswerCount)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close TCP client: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close TCP listener: %v", err)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("serveTCP returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveTCP did not stop after its listener was closed")
	}
}

func TestServeTCPHandlesConnectionsConcurrently(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for TCP: %v", err)
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveTCP(listener, testZone(), io.Discard)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-serverDone:
			if err != nil {
				t.Errorf("serveTCP returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("serveTCP did not stop after its listener was closed")
		}
	})

	idleConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial idle TCP client: %v", err)
	}
	t.Cleanup(func() { _ = idleConn.Close() })

	activeConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial active TCP client: %v", err)
	}
	t.Cleanup(func() { _ = activeConn.Close() })
	if err := activeConn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set active TCP deadline: %v", err)
	}

	response := exchangeTestTCPMessage(t, activeConn, sampleQueryWithTypeClass(TypeA, ClassIN))
	if answerCount := binary.BigEndian.Uint16(response[6:8]); answerCount != 1 {
		t.Fatalf("ANCOUNT = %d, want 1", answerCount)
	}
}

func TestHandleTCPConnectionRejectsIncompleteFrame(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- handleTCPConnection(serverConn, testZone(), io.Discard)
	}()

	if _, err := clientConn.Write([]byte{0x00, 0x04, 0x12, 0x34}); err != nil {
		t.Fatalf("write incomplete TCP frame: %v", err)
	}
	if err := clientConn.Close(); err != nil {
		t.Fatalf("close TCP client: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("handleTCPConnection returned nil error for incomplete frame")
		}
	case <-time.After(time.Second):
		t.Fatal("handleTCPConnection did not return after incomplete frame")
	}
}

func exchangeTestTCPMessage(t *testing.T, conn net.Conn, query []byte) []byte {
	t.Helper()

	frame := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(frame[0:2], uint16(len(query)))
	copy(frame[2:], query)
	if err := writeTestBytes(conn, frame); err != nil {
		t.Fatalf("write TCP query: %v", err)
	}

	var lengthBytes [2]byte
	if _, err := io.ReadFull(conn, lengthBytes[:]); err != nil {
		t.Fatalf("read TCP response length: %v", err)
	}
	response := make([]byte, int(binary.BigEndian.Uint16(lengthBytes[:])))
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("read TCP response: %v", err)
	}

	return response
}

func writeTestBytes(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}

	return nil
}
