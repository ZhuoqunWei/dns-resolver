package main

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestRunServersStopsBothTransportsOnCancellation(t *testing.T) {
	udpConn, tcpListener := listenTestDNSTransports(t)
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		runDone <- runServers(ctx, udpConn, tcpListener, testZone(), io.Discard)
		close(finished)
	}()
	t.Cleanup(func() {
		cancel()
		_ = udpConn.Close()
		_ = tcpListener.Close()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("runServers did not stop during test cleanup")
		}
	})

	udpClient, err := net.DialUDP("udp", nil, udpConn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial UDP server: %v", err)
	}
	defer udpClient.Close()
	if err := udpClient.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set UDP deadline: %v", err)
	}
	query := sampleQueryWithTypeClass(TypeA, ClassIN)
	if _, err := udpClient.Write(query); err != nil {
		t.Fatalf("write UDP query: %v", err)
	}
	udpResponse := make([]byte, 512)
	n, err := udpClient.Read(udpResponse)
	if err != nil {
		t.Fatalf("read UDP response: %v", err)
	}
	if n < HeaderSize {
		t.Fatalf("UDP response length = %d, want at least %d", n, HeaderSize)
	}
	if answerCount := binary.BigEndian.Uint16(udpResponse[6:8]); answerCount != 1 {
		t.Fatalf("UDP ANCOUNT = %d, want 1", answerCount)
	}

	tcpClient, err := net.Dial("tcp", tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("dial TCP server: %v", err)
	}
	defer tcpClient.Close()
	if err := tcpClient.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set TCP deadline: %v", err)
	}
	tcpResponse := exchangeTestTCPMessage(t, tcpClient, query)
	if answerCount := binary.BigEndian.Uint16(tcpResponse[6:8]); answerCount != 1 {
		t.Fatalf("TCP ANCOUNT = %d, want 1", answerCount)
	}

	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("runServers returned error during cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runServers did not stop after cancellation")
	}

	if err := tcpClient.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set post-shutdown TCP deadline: %v", err)
	}
	var oneByte [1]byte
	if _, err := tcpClient.Read(oneByte[:]); err == nil {
		t.Fatal("TCP client read succeeded after server shutdown")
	} else {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			t.Fatalf("TCP connection remained open after server shutdown: %v", err)
		}
	}
}

func TestRunServersStopsPeerTransportAfterServerFailure(t *testing.T) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatalf("listen for UDP: %v", err)
	}
	defer udpConn.Close()

	acceptErr := errors.New("test accept failure")
	err = runServers(
		context.Background(),
		udpConn,
		failingListener{err: acceptErr},
		testZone(),
		io.Discard,
	)
	if !errors.Is(err, acceptErr) {
		t.Fatalf("runServers error = %v, want wrapped accept error", err)
	}

	if _, _, err := udpConn.ReadFromUDP(make([]byte, 1)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("UDP read after peer failure returned %v, want net.ErrClosed", err)
	}
}

func listenTestDNSTransports(t *testing.T) (*net.UDPConn, net.Listener) {
	t.Helper()

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for TCP: %v", err)
	}
	tcpAddr := tcpListener.Addr().(*net.TCPAddr)
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: tcpAddr.Port,
	})
	if err != nil {
		_ = tcpListener.Close()
		t.Fatalf("listen for UDP: %v", err)
	}

	return udpConn, tcpListener
}

type failingListener struct {
	err error
}

func (l failingListener) Accept() (net.Conn, error) {
	return nil, l.err
}

func (failingListener) Close() error {
	return nil
}

func (failingListener) Addr() net.Addr {
	return &net.TCPAddr{}
}
