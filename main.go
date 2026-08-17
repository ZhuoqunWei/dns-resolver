package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type serverResult struct {
	name string
	err  error
}

func main() {
	const listenAddress = "127.0.0.1:8053"

	zone, err := loadZone("records.json")
	if err != nil {
		log.Fatal("load zone: ", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer udpConn.Close()

	tcpListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		_ = udpConn.Close()
		log.Fatal(err)
	}
	defer tcpListener.Close()

	fmt.Println("DNS server listening on 127.0.0.1:8053 over UDP and TCP")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runServers(ctx, udpConn, tcpListener, zone, os.Stdout); err != nil {
		log.Print(err)
		return
	}

	fmt.Println("DNS server stopped")
}

func runServers(
	ctx context.Context,
	udpConn *net.UDPConn,
	tcpListener net.Listener,
	zone Zone,
	output io.Writer,
) error {
	results := make(chan serverResult, 2)
	go func() {
		results <- serverResult{name: "UDP", err: serveUDP(udpConn, zone, output)}
	}()
	go func() {
		results <- serverResult{name: "TCP", err: serveTCP(tcpListener, zone, output)}
	}()

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			_ = udpConn.Close()
			_ = tcpListener.Close()
		})
	}

	ctxDone := ctx.Done()
	var firstErr error
	for completed := 0; completed < 2; {
		select {
		case <-ctxDone:
			shutdown()
			ctxDone = nil
		case result := <-results:
			completed++
			if result.err != nil && firstErr == nil {
				firstErr = fmt.Errorf("%s server: %w", result.name, result.err)
			}
			shutdown()
		}
	}

	return firstErr
}
