package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {

	// UDP server
	// 1. Choose the address the UDP server will listen on
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8053")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Start listening on UDP
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Println("DNS UDP server listening on 127.0.0.1:8053")

	records := map[string]ARecord{
		"example.com": {
			Address: [4]byte{1, 2, 3, 4},
			TTL:     60,
		},
		"test.local": {
			Address: [4]byte{5, 6, 7, 8},
			TTL:     60,
		},
	}

	if err := serveUDP(conn, records, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
