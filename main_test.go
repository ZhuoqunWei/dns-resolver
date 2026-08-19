package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"io"
	"net"
	"strings"
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

func TestParseServerOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want serverOptions
	}{
		{
			name: "defaults",
			want: serverOptions{
				listenAddress: defaultListenAddress,
				configPath:    defaultConfigPath,
			},
		},
		{
			name: "overrides",
			args: []string{"-listen", "127.0.0.1:9053", "-config", "testdata/zone.json"},
			want: serverOptions{
				listenAddress: "127.0.0.1:9053",
				configPath:    "testdata/zone.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServerOptions(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("parseServerOptions() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseServerOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseServerOptionsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "unknown flag", args: []string{"-unknown"}, wantMessage: "flag provided but not defined"},
		{name: "missing flag value", args: []string{"-listen"}, wantMessage: "flag needs an argument"},
		{name: "positional argument", args: []string{"records.json"}, wantMessage: "unexpected positional arguments"},
		{name: "empty listen address", args: []string{"-listen", ""}, wantMessage: "listen address must not be empty"},
		{name: "empty config path", args: []string{"-config", "  "}, wantMessage: "config path must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseServerOptions(tt.args, io.Discard)
			if err == nil {
				t.Fatal("parseServerOptions() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("parseServerOptions() error = %q, want message containing %q", err, tt.wantMessage)
			}
		})
	}
}

func TestParseServerOptionsPrintsHelp(t *testing.T) {
	var output strings.Builder
	_, err := parseServerOptions([]string{"-h"}, &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseServerOptions() error = %v, want flag.ErrHelp", err)
	}
	if help := output.String(); !strings.Contains(help, "-listen") || !strings.Contains(help, "-config") {
		t.Fatalf("help output = %q, want listen and config options", help)
	}
}

func TestListenDNSTransportsUsesSameAddress(t *testing.T) {
	udpConn, tcpListener, err := listenDNSTransports("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenDNSTransports() error = %v", err)
	}
	defer udpConn.Close()
	defer tcpListener.Close()

	if udpConn.LocalAddr().String() != tcpListener.Addr().String() {
		t.Fatalf(
			"UDP address = %s, TCP address = %s, want equal addresses",
			udpConn.LocalAddr(),
			tcpListener.Addr(),
		)
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

	udpConn, tcpListener, err := listenDNSTransports("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for test DNS transports: %v", err)
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
