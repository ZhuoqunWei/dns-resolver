package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	defaultListenAddress = "127.0.0.1:8053"
	defaultConfigPath    = "records.json"
)

type serverOptions struct {
	listenAddress string
	configPath    string
}

type serverResult struct {
	name string
	err  error
}

func main() {
	options, err := parseServerOptions(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal("parse options: ", err)
	}

	zone, err := loadZone(options.configPath)
	if err != nil {
		log.Fatal("load zone: ", err)
	}

	udpConn, tcpListener, err := listenDNSTransports(options.listenAddress)
	if err != nil {
		log.Fatal("listen: ", err)
	}
	defer udpConn.Close()
	defer tcpListener.Close()

	fmt.Printf("DNS server listening on %s over UDP and TCP\n", udpConn.LocalAddr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runServers(ctx, udpConn, tcpListener, zone, os.Stdout); err != nil {
		log.Print(err)
		return
	}

	fmt.Println("DNS server stopped")
}

func parseServerOptions(args []string, output io.Writer) (serverOptions, error) {
	options := serverOptions{
		listenAddress: defaultListenAddress,
		configPath:    defaultConfigPath,
	}
	var flagOutput bytes.Buffer
	flags := flag.NewFlagSet("dns-resolver", flag.ContinueOnError)
	flags.SetOutput(&flagOutput)
	flags.StringVar(&options.listenAddress, "listen", options.listenAddress, "UDP and TCP listen address")
	flags.StringVar(&options.configPath, "config", options.configPath, "path to the zone configuration file")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.Copy(output, &flagOutput)
		}
		return serverOptions{}, err
	}
	if flags.NArg() > 0 {
		return serverOptions{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(options.listenAddress) == "" {
		return serverOptions{}, fmt.Errorf("listen address must not be empty")
	}
	if strings.TrimSpace(options.configPath) == "" {
		return serverOptions{}, fmt.Errorf("config path must not be empty")
	}

	return options, nil
}

func listenDNSTransports(listenAddress string) (*net.UDPConn, net.Listener, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve UDP address %q: %w", listenAddress, err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen for UDP on %q: %w", listenAddress, err)
	}

	tcpListener, err := net.Listen("tcp", udpConn.LocalAddr().String())
	if err != nil {
		_ = udpConn.Close()
		return nil, nil, fmt.Errorf("listen for TCP on %q: %w", udpConn.LocalAddr(), err)
	}

	return udpConn, tcpListener, nil
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
